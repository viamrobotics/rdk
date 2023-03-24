package robotimpl

import (
	"context"
	"crypto/tls"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module/modmanager"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

func init() {
	if err := cleanAppImageEnv(); err != nil {
		golog.Global().Errorw("error cleaning up app image environement", "error", err)
	}
}

const (
	remoteTypeName  = resource.TypeName("remote")
	unknownTypeName = resource.TypeName("unk")
)

var remoteSubtype = resource.NewSubtype(resource.ResourceNamespaceRDK,
	remoteTypeName,
	resource.SubtypeName(""))

var unknownSubtype = resource.NewSubtype(resource.ResourceNamespaceRDK,
	unknownTypeName,
	resource.SubtypeName(""))

var (
	errShellServiceDisabled = errors.New("shell service disabled in an untrusted environment")
	errProcessesDisabled    = errors.New("processes disabled in an untrusted environment")
)

type translateToName func(string) (resource.Name, bool)

// resourceManager manages the actual parts that make up a robot.
type resourceManager struct {
	resources      *resource.Graph
	processManager pexec.ProcessManager
	opts           resourceManagerOptions
	logger         golog.Logger
	configLock     *sync.Mutex
}

// resourcePlaceholder we use resourcePlaceholder during a reconfiguration
// it holds the former resource interface (nil if added) and it's most recent configuration.
type resourcePlaceholder struct {
	real   interface{}
	config interface{}
	err    error
}

type resourceManagerOptions struct {
	debug              bool
	fromCommand        bool
	allowInsecureCreds bool
	untrustedEnv       bool
	tlsConfig          *tls.Config
}

func (w *resourcePlaceholder) Close(ctx context.Context) error {
	if w.real != nil {
		return utils.TryClose(ctx, w.real)
	}
	return nil
}

// newResourceManager returns a properly initialized set of parts.
func newResourceManager(
	opts resourceManagerOptions,
	logger golog.Logger,
) *resourceManager {
	var processManager pexec.ProcessManager
	if opts.untrustedEnv {
		processManager = pexec.NoopProcessManager
	} else {
		processManager = pexec.NewProcessManager(logger)
	}

	return &resourceManager{
		resources:      resource.NewGraph(),
		processManager: processManager,
		opts:           opts,
		logger:         logger,
		configLock:     &sync.Mutex{},
	}
}

func fromRemoteNameToRemoteNodeName(name string) resource.Name {
	return resource.NameFromSubtype(remoteSubtype, name)
}

// addRemote adds a remote to the manager.
func (manager *resourceManager) addRemote(ctx context.Context, rr robot.Robot, c config.Remote, r *localRobot) {
	rName := fromRemoteNameToRemoteNodeName(c.Name)
	manager.addResource(rName, rr)
	manager.updateRemoteResourceNames(ctx, rName, rr, r)
}

func (manager *resourceManager) remoteResourceNames(remoteName resource.Name) []resource.Name {
	var filtered []resource.Name
	if _, ok := manager.resources.Node(remoteName); !ok {
		manager.logger.Errorw("trying to get remote resources of a non existing remote", "remote", remoteName)
	}
	children := manager.resources.GetAllChildrenOf(remoteName)
	for _, child := range children {
		if child.ContainsRemoteNames() {
			filtered = append(filtered, child)
		}
	}
	return filtered
}

// updateRemoteResourceNames this function is called when the Remote robot has changed (either connection or disconnection)
// it will pull the current remote resources and update the resource tree adding or removing nodes accordingly
// If any local resources are dependent on a remote resource two things can happen
// 1) The remote resource already is in the tree as an unknown type, it will be renamed
// 2) A remote resource is being deleted but a local resource depends on it.
// It will be renamed as unknown and its local children are going to be destroyed.
func (manager *resourceManager) updateRemoteResourceNames(
	ctx context.Context,
	remoteName resource.Name,
	rr robot.Robot,
	lr *localRobot,
) bool {
	visited := map[resource.Name]bool{}
	newResources := rr.ResourceNames()
	oldResources := manager.remoteResourceNames(remoteName)
	for _, res := range oldResources {
		visited[res] = false
	}

	anythingChanged := false

	for _, res := range newResources {
		rrName := res
		res = res.PrependRemote(resource.RemoteName(remoteName.Name))
		if _, ok := visited[res]; ok {
			visited[res] = true
			continue
		}
		iface, err := rr.ResourceByName(rrName) // this returns a remote known OR foreign resource client
		if err != nil {
			if errors.Is(err, client.ErrMissingClientRegistration) {
				manager.logger.Debugw("couldn't obtain remote resource interface",
					"name", rrName,
					"reason", err)
			} else {
				manager.logger.Errorw("couldn't obtain remote resource interface",
					"name", rrName,
					"reason", err)
			}
			continue
		}
		asUnknown := resource.NewName(resource.ResourceNamespaceRDK,
			unknownTypeName,
			resource.SubtypeName(""), res.Name)
		asUnknown = asUnknown.PrependRemote(res.Remote)
		if _, ok := visited[asUnknown]; ok {
			manager.logger.Infow("we have an unknown res that we are converting to", "unknown", asUnknown, "new name", res)
			visited[asUnknown] = true
			if err := manager.resources.RenameNode(asUnknown, res); err != nil {
				manager.logger.Errorw("fail to rename node", "node", asUnknown, "error", err)
			}
		}
		manager.addResource(res, iface)
		err = manager.resources.AddChildren(res, remoteName)
		if err != nil {
			manager.logger.Errorw("error while trying add node as a dependency of remote", "node", res, "remote", remoteName)
		} else {
			anythingChanged = true
		}
	}
	for res, visit := range visited {
		if !visit {
			manager.logger.Debugf("deleting res %q", res)
			err := manager.markChildrenForUpdate(ctx, res, lr)
			if err != nil {
				manager.logger.Errorw("failed to mark children of remote for update", "resource", res, "reason", err)
				continue
			}
			if len(manager.resources.GetAllChildrenOf(res)) > 0 {
				asUnknown := resource.NewName(resource.ResourceNamespaceRDK,
					unknownTypeName,
					resource.SubtypeName(""), res.Name)
				asUnknown = asUnknown.PrependRemote(res.Remote)
				if err := manager.resources.RenameNode(res, asUnknown); err != nil {
					manager.logger.Errorw("error while renaming node", "error", err)
				}
				manager.resources.AddNode(asUnknown, nil)
				continue
			}
			manager.resources.Remove(res)
			anythingChanged = true
		}
	}
	return anythingChanged
}

func (manager *resourceManager) updateRemotesResourceNames(ctx context.Context, r *localRobot) bool {
	anythingChanged := false
	for _, name := range manager.resources.Names() {
		iface, _ := manager.resources.Node(name)
		if name.ResourceType == remoteTypeName {
			if rr, ok := iface.(robot.Robot); ok {
				anythingChanged = anythingChanged || manager.updateRemoteResourceNames(ctx, name, rr, r)
			}
		}
	}
	return anythingChanged
}

// addResource adds a resource to the manager.
func (manager *resourceManager) addResource(name resource.Name, r interface{}) {
	manager.resources.AddNode(name, r)
}

// RemoteNames returns the names of all remotes in the manager.
func (manager *resourceManager) RemoteNames() []string {
	names := []string{}
	for _, k := range manager.resources.Names() {
		iface, _ := manager.resources.Node(k)
		if k.ResourceType == remoteTypeName && iface != nil {
			names = append(names, k.Name)
		}
	}
	return names
}

func (manager *resourceManager) anyResourcesNotConfigured() bool {
	for _, name := range manager.resources.Names() {
		iface, ok := manager.resources.Node(name)
		if !ok {
			continue
		}
		if _, ok := iface.(*resourcePlaceholder); ok || iface == nil {
			return true
		}
	}
	return false
}

// ResourceNames returns the names of all resources in the manager.
func (manager *resourceManager) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for _, k := range manager.resources.Names() {
		if k.ResourceType == remoteTypeName {
			continue
		}
		iface, ok := manager.resources.Node(k)
		if !ok {
			continue
		}
		if _, ok := iface.(*resourcePlaceholder); ok || iface == nil {
			continue
		}
		names = append(names, k)
	}
	return names
}

// ResourceRPCSubtypes returns the types of all resource RPC subtypes in use by the manager.
func (manager *resourceManager) ResourceRPCSubtypes() []resource.RPCSubtype {
	resourceSubtypes := registry.RegisteredResourceSubtypes()

	types := map[resource.Subtype]*desc.ServiceDescriptor{}
	for _, k := range manager.resources.Names() {
		if k.ResourceType == remoteTypeName {
			iface, ok := manager.resources.Node(k)
			if !ok {
				continue
			}
			rr, ok := iface.(robot.Robot)
			if !ok {
				if _, ok := iface.(*resourcePlaceholder); !ok {
					manager.logger.Debugw(
						"remote does not implement robot interface and is not a resource placeholder",
						"remote",
						k.Name,
						"type",
						reflect.TypeOf(iface),
					)
				}
				continue
			}
			manager.mergeResourceRPCSubtypesWithRemote(rr, types)
			continue
		}
		if k.ContainsRemoteNames() {
			continue
		}
		if types[k.Subtype] != nil {
			continue
		}

		st, ok := resourceSubtypes[k.Subtype]
		if !ok {
			continue
		}

		if st.ReflectRPCServiceDesc != nil {
			types[k.Subtype] = st.ReflectRPCServiceDesc
		}
	}
	typesList := make([]resource.RPCSubtype, 0, len(types))
	for k, v := range types {
		typesList = append(typesList, resource.RPCSubtype{
			Subtype: k,
			Desc:    v,
		})
	}
	return typesList
}

// mergeResourceRPCSubtypesWithRemotes merges types from the manager itself as well as its
// remotes.
func (manager *resourceManager) mergeResourceRPCSubtypesWithRemote(r robot.Robot, types map[resource.Subtype]*desc.ServiceDescriptor) {
	remoteTypes := r.ResourceRPCSubtypes()
	for _, remoteType := range remoteTypes {
		if svcName, ok := types[remoteType.Subtype]; ok {
			if svcName.GetFullyQualifiedName() != remoteType.Desc.GetFullyQualifiedName() {
				manager.logger.Errorw(
					"remote proto service name clashes with another of same subtype",
					"existing", svcName.GetFullyQualifiedName(),
					"remote", remoteType.Desc.GetFullyQualifiedName())
			}
			continue
		}
		types[remoteType.Subtype] = remoteType.Desc
	}
}

// Close attempts to close/stop all parts.
func (manager *resourceManager) Close(ctx context.Context, r *localRobot) error {
	var allErrs error
	if err := manager.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error stopping process manager"))
	}

	order := manager.resources.TopologicalSort()
	for _, x := range order {
		iface, ok := manager.resources.Node(x)
		if !ok {
			continue
		}

		if err := utils.TryClose(ctx, iface); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing resource"))
		}

		if r.modules != nil && r.ModuleManager().IsModularResource(x) {
			if err := r.ModuleManager().RemoveResource(ctx, x); err != nil {
				allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error removing modular resource"))
			}
		}
	}
	return allErrs
}

// completeConfig process the tree in reverse order and attempts to build
// or reconfigure resources that are wrapped in a placeholderResource.
func (manager *resourceManager) completeConfig(
	ctx context.Context,
	robot *localRobot,
) {
	manager.configLock.Lock()
	defer manager.configLock.Unlock()
	rS := manager.resources.ReverseTopologicalSort()

	for _, r := range rS {
		iface, ok := manager.resources.Node(r)
		if !ok || iface == nil {
			continue
		}
		wrap, ok := iface.(*resourcePlaceholder)
		if !ok {
			continue
		}
		manager.logger.Debugw("we are now handling the resource", "resource", r)
		if c, ok := wrap.config.(config.Component); ok {
			// Check for Validation errors.
			_, err := c.Validate("")
			if err != nil {
				manager.logger.Errorw("component config validation error", "resource", c.ResourceName(), "model", c.Model, "error", err)
				wrap.err = errors.Wrap(err, "config validation error found in component: "+c.Name)
				continue
			}
			// Check for modular Validation errors.
			if robot.ModuleManager().Provides(c) {
				_, err = robot.ModuleManager().ValidateConfig(ctx, c)
				if err != nil {
					manager.logger.Errorw("modular component config validation error", "resource", c.ResourceName(), "model", c.Model, "error", err)
					wrap.err = errors.Wrap(err, "config validation error found in modular component: "+c.Name)
					continue
				}
			}

			// TODO(PRODUCT-266): "r" isn't likely needed here, as c.ResourceName() should be the same.
			iface, err := manager.processComponent(ctx, r, c, wrap.real, robot)
			if err != nil {
				manager.logger.Errorw("error building component", "resource", c.ResourceName(), "model", c.Model, "error", err)
				wrap.err = errors.Wrap(err, "component build error")
				continue
			}
			manager.resources.AddNode(r, iface)
		} else if s, ok := wrap.config.(config.Service); ok {
			// Check for Validation errors.
			_, err := s.Validate("")
			if err != nil {
				manager.logger.Errorw("service config validation error", "resource", s.ResourceName(), "model", s.Model, "error", err)
				wrap.err = errors.Wrap(err, "config validation error found in service: "+s.Name)
				continue
			}
			// Check for modular Validation errors.
			sCfg := config.ServiceConfigToShared(s)
			if robot.ModuleManager().Provides(sCfg) {
				_, err = robot.ModuleManager().ValidateConfig(ctx, sCfg)
				if err != nil {
					manager.logger.Errorw("modular service config validation error", "resource", s.ResourceName(), "model", s.Model, "error", err)
					wrap.err = errors.Wrap(err, "config validation error found in modular service: "+sCfg.Name)
					continue
				}
			}

			iface, err := manager.processService(ctx, s, wrap.real, robot)
			if err != nil {
				manager.logger.Errorw("error building service", "resource", s.ResourceName(), "model", s.Model, "error", err)
				wrap.err = errors.Wrap(err, "service build error")
				continue
			}
			manager.resources.AddNode(r, iface)
		} else if rc, ok := wrap.config.(config.Remote); ok {
			err := rc.Validate("")
			if err != nil {
				manager.logger.Errorw("remote config validation error", "remote", rc.Name, "error", err)
				wrap.err = errors.Wrap(err, "config validation error found in remote: "+rc.Name)
				continue
			}
			rr, err := manager.processRemote(ctx, rc)
			if err != nil {
				manager.logger.Errorw("error connecting to remote", "remote", rc.Name, "error", err)
				wrap.err = errors.Wrap(err, "remote connection error")
				continue
			}
			manager.addRemote(ctx, rr, rc, robot)
			rr.SetParentNotifier(func() {
				rName := rc.Name
				if robot.closeContext.Err() != nil {
					return
				}
				select {
				case <-robot.closeContext.Done():
					return
				case robot.remotesChanged <- rName:
				}
			})
		} else {
			err := errors.New("config is not a component, service, or remote config")
			manager.logger.Errorw(err.Error(), "resource", r)
		}
	}
}

// cleanAppImageEnv attempts to revert environment variable changes so
// normal, non-AppImage processes can be executed correctly.
func cleanAppImageEnv() error {
	_, isAppImage := os.LookupEnv("APPIMAGE")
	if isAppImage {
		err := os.Chdir(os.Getenv("APPRUN_CWD"))
		if err != nil {
			return err
		}

		// Reset original values where available
		for _, eVarStr := range os.Environ() {
			eVar := strings.Split(eVarStr, "=")
			key := eVar[0]
			origV, present := os.LookupEnv("APPRUN_ORIGINAL_" + key)
			if present {
				if origV != "" {
					err = os.Setenv(key, origV)
				} else {
					err = os.Unsetenv(key)
				}
				if err != nil {
					return err
				}
			}
		}

		// Remove all explicit appimage vars
		err = multierr.Combine(os.Unsetenv("ARGV0"), os.Unsetenv("ORIGIN"))
		for _, eVarStr := range os.Environ() {
			eVar := strings.Split(eVarStr, "=")
			key := eVar[0]
			if strings.HasPrefix(key, "APPRUN") ||
				strings.HasPrefix(key, "APPDIR") ||
				strings.HasPrefix(key, "APPIMAGE") ||
				strings.HasPrefix(key, "AIX_") {
				err = multierr.Combine(err, os.Unsetenv(key))
			}
		}
		if err != nil {
			return err
		}

		// Remove AppImage paths from path-like env vars
		for _, eVarStr := range os.Environ() {
			eVar := strings.Split(eVarStr, "=")
			var newPaths []string
			const mountPrefix = "/tmp/.mount_"
			key := eVar[0]
			if len(eVar) >= 2 && strings.Contains(eVar[1], mountPrefix) {
				for _, path := range strings.Split(eVar[1], ":") {
					if !strings.HasPrefix(path, mountPrefix) && path != "" {
						newPaths = append(newPaths, path)
					}
				}
				if len(newPaths) > 0 {
					err = os.Setenv(key, strings.Join(newPaths, ":"))
				} else {
					err = os.Unsetenv(key)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (manager *resourceManager) processRemote(ctx context.Context,
	config config.Remote,
) (*client.RobotClient, error) {
	dialOpts := remoteDialOptions(config, manager.opts)
	manager.logger.Debugw("connecting now to remote", "remote", config.Name)
	robotClient, err := dialRobotClient(ctx, config, manager.logger, dialOpts...)
	if err != nil {
		if errors.Is(err, rpc.ErrInsecureWithCredentials) {
			if manager.opts.fromCommand {
				err = errors.New("must use -allow-insecure-creds flag to connect to a non-TLS secured robot")
			} else {
				err = errors.New("must use Config.AllowInsecureCreds to connect to a non-TLS secured robot")
			}
		}
		return nil, errors.Errorf("couldn't connect to robot remote (%s): %s", config.Address, err)
	}
	manager.logger.Debugw("connected now to remote", "remote", config.Name)
	return robotClient, nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (manager *resourceManager) RemoteByName(name string) (robot.Robot, bool) {
	rName := resource.NameFromSubtype(remoteSubtype, name)
	if iface, ok := manager.resources.Node(rName); ok {
		part, ok := iface.(robot.Robot)
		if !ok {
			if ph, ok := iface.(*resourcePlaceholder); ok {
				manager.logger.Errorw("remote not available", "remote", name, "err", ph.err)
			} else {
				manager.logger.Errorw("tried to access remote but its not a robot interface", "remote", name, "type", reflect.TypeOf(iface))
			}
		}
		return part, ok
	}
	return nil, false
}

func (manager *resourceManager) processService(ctx context.Context,
	c config.Service,
	old interface{},
	robot *localRobot,
) (interface{}, error) {
	if old == nil {
		return robot.newService(ctx, c)
	}

	if robot.ModuleManager().Provides(config.ServiceConfigToShared(c)) {
		deps, err := robot.getDependencies(c.ResourceName())
		if err != nil {
			return nil, err
		}
		return old, robot.ModuleManager().ReconfigureResource(ctx, config.ServiceConfigToShared(c), modmanager.DepsToNames(deps))
	}

	svc, err := robot.newService(ctx, c)
	if err != nil {
		return nil, err
	}
	return resource.ReconfigureResource(ctx, old, svc)
}

func (manager *resourceManager) markChildrenForUpdate(ctx context.Context, rName resource.Name, r *localRobot) error {
	sg, err := manager.resources.SubGraphFrom(rName)
	if err != nil {
		return err
	}
	sorted := sg.TopologicalSort()
	for _, x := range sorted {
		var originalConfig config.Component
		iface, _ := manager.resources.Node(x)
		if _, ok := iface.(*resourcePlaceholder); ok {
			continue
		}
		if x.ContainsRemoteNames() {
			continue // ignore non-local resources
		}
		if r != nil {
			for _, c := range r.config.Components {
				if c.ResourceName() == x {
					originalConfig = c
				}
			}
		}
		if err := utils.TryClose(ctx, iface); err != nil {
			return err
		}
		wrapper := &resourcePlaceholder{
			real:   nil,
			config: originalConfig,
			err:    errors.New("resource not updated yet"),
		}
		manager.resources.AddNode(x, wrapper)
	}
	return nil
}

func (manager *resourceManager) processComponent(ctx context.Context,
	rName resource.Name,
	conf config.Component,
	old interface{},
	r *localRobot,
) (interface{}, error) {
	if old == nil {
		return r.newResource(ctx, conf)
	}
	res := config.Rebuild
	if r.ModuleManager().Provides(conf) {
		deps, err := r.getDependencies(rName)
		if err != nil {
			return nil, err
		}
		err = r.ModuleManager().ReconfigureResource(ctx, conf, modmanager.DepsToNames(deps))
		if err != nil {
			return nil, err
		}
		res = config.None
	} else {
		obj, canValidate := old.(config.ComponentUpdate)
		if canValidate {
			res = obj.UpdateAction(&conf)
		}
	}
	switch res {
	case config.None:
		return old, nil
	case config.Reconfigure:
		if err := manager.markChildrenForUpdate(ctx, rName, r); err != nil {
			return old, err
		}
		nr, err := r.newResource(ctx, conf)
		if err != nil {
			return old, err
		}
		rr, err := resource.ReconfigureResource(ctx, old, nr)
		if err != nil {
			return old, err
		}
		return rr, nil
	case config.Rebuild:
		if err := manager.markChildrenForUpdate(ctx, rName, r); err != nil {
			return old, err
		}
		if err := utils.TryClose(ctx, old); err != nil {
			return old, err
		}
		nr, err := r.newResource(ctx, conf)
		if err != nil {
			return old, err
		}
		return nr, nil
	default:
		return old, errors.New("un-handeled case of reconfigure action")
	}
}

// wrapResource creates a resourcePlaceholder associating the former resource object (if it exists) and it's configuration.
// It will also look for dependencies and link them properly.
// once done we should have all the information we need to build this resource later on when we call completeConfig.
func (manager *resourceManager) wrapResource(name resource.Name, config interface{}, deps []string, fn translateToName) error {
	var wrapper *resourcePlaceholder
	part, _ := manager.resources.Node(name)
	if wrap, ok := part.(*resourcePlaceholder); ok {
		wrap.config = config
		wrapper = wrap
	} else {
		wrapper = &resourcePlaceholder{
			real:   part,
			config: config,
			err:    errors.New("resource not initialized yet"),
		}
	}
	// the first thing we need to do is seek if the resource name already exists as an unknownType, if so
	// we need to replace it
	if old, ok := manager.resources.FindNodeByName(name.Name); ok && old.ResourceType == unknownTypeName {
		manager.logger.Errorw("renaming resource", "old", old, "new", name)
		if err := manager.resources.RenameNode(*old, name); err != nil {
			manager.logger.Errorw("error renaming a resource", "error", err)
		}
	}
	manager.addResource(name, wrapper)
	parents := manager.resources.GetAllParentsOf(name)
	mapParents := make(map[resource.Name]bool)
	for _, pdep := range parents {
		mapParents[pdep] = false
	}
	for _, dep := range deps {
		var parent resource.Name
		if p, ok := fn(dep); ok {
			parent = p
		} else if p, ok := manager.resources.FindNodeByName(dep); ok {
			parent = *p
		} else {
			manager.logger.Errorw("the dependency for resource  doesn't exist, it will not be added", "dependency", dep, "resource", name)
			continue
		}
		if _, ok := mapParents[parent]; ok {
			mapParents[parent] = true
			continue
		}
		if parent.ContainsRemoteNames() {
			// when a local resource depends on a remote then it's possible the remote wasn't added yet.
			// it's ok, it will be added by addChildren and properly configured later on
			remote, _ := remoteNameByResource(parent)
			if err := manager.resources.AddChildren(parent,
				fromRemoteNameToRemoteNodeName(remote)); err != nil {
				manager.logger.Errorw("cannot add remote dependency", "error", err)
			}
		}
		if err := manager.resources.AddChildren(name, parent); err != nil {
			manager.logger.Errorw("cannot add dependency", "error", err)
			manager.resources.Remove(name)
			return err
		}
	}
	for k, v := range mapParents {
		if v {
			continue
		}
		manager.resources.RemoveChildren(name, k)
	}
	return nil
}

// updateResources using the difference between current config
// and next we create resource wrappers to be consumed by completeConfig later on
// Ideally at the end of this function we should have a complete graph representation of the configuration.
func (manager *resourceManager) updateResources(
	ctx context.Context,
	config *config.Diff,
	fn translateToName,
) error {
	manager.configLock.Lock()
	defer manager.configLock.Unlock()
	var allErrs error
	for _, c := range config.Added.Components {
		rName := c.ResourceName()
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, c, c.Dependencies(), fn))
	}
	for _, s := range config.Added.Services {
		rName := s.ResourceName()
		if manager.opts.untrustedEnv && rName.Subtype == shell.Subtype {
			allErrs = multierr.Combine(allErrs, errShellServiceDisabled)
			continue
		}
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, s, s.Dependencies(), fn))
	}
	for _, r := range config.Added.Remotes {
		rName := fromRemoteNameToRemoteNodeName(r.Name)
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, r, []string{}, fn))
	}
	for _, c := range config.Modified.Components {
		rName := c.ResourceName()
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, c, c.Dependencies(), fn))
	}
	for _, s := range config.Modified.Services {
		rName := s.ResourceName()

		// Disable shell service when in untrusted env
		if manager.opts.untrustedEnv && rName.Subtype == shell.Subtype {
			allErrs = multierr.Combine(allErrs, errShellServiceDisabled)
			continue
		}

		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, s, s.Dependencies(), fn))
	}
	for _, r := range config.Modified.Remotes {
		rName := fromRemoteNameToRemoteNodeName(r.Name)
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, r, []string{}, fn))
	}
	// processes are not added into the resource tree as they belong to a process manager

	for _, p := range config.Added.Processes {
		if manager.opts.untrustedEnv {
			allErrs = multierr.Combine(allErrs, errProcessesDisabled)
			break
		}

		_, err := manager.processManager.AddProcessFromConfig(ctx, p)
		if err != nil {
			manager.logger.Errorw("error while adding process, skipping", "process", p.ID, "error", err)
			continue
		}
	}
	for _, p := range config.Modified.Processes {
		if manager.opts.untrustedEnv {
			allErrs = multierr.Combine(allErrs, errProcessesDisabled)
			break
		}

		if oldProc, ok := manager.processManager.RemoveProcessByID(p.ID); ok {
			if err := oldProc.Stop(); err != nil {
				manager.logger.Errorw("couldn't stop process", "process", p.ID, "error", err)
			}
		} else {
			manager.logger.Errorw("couldn't find modified process", "process", p.ID)
		}
		_, err := manager.processManager.AddProcessFromConfig(ctx, p)
		if err != nil {
			manager.logger.Errorw("error while changing process, skipping", "process", p.ID, "error", err)
			continue
		}
	}
	return allErrs
}

// ResourceByName returns the given resource by fully qualified name, if it exists;
// returns an error otherwise.
func (manager *resourceManager) ResourceByName(name resource.Name) (interface{}, error) {
	robotPart, ok := manager.resources.Node(name)
	if ok && robotPart != nil {
		ph, ok := robotPart.(*resourcePlaceholder)
		if !ok {
			return robotPart, nil
		}
		return nil, rutils.NewResourceNotAvailableError(name, ph.err)
	}
	// if we haven't found a resource of this name then we are going to look into remote resources to find it.
	if !ok && !name.ContainsRemoteNames() {
		keys := manager.resources.FindNodesByShortNameAndSubtype(name)
		if len(keys) > 1 {
			return nil, rutils.NewRemoteResourceClashError(name.Name)
		}
		if len(keys) == 1 {
			robotPart, _ := manager.resources.Node(keys[0])
			return robotPart, nil
		}
	}
	return nil, rutils.NewResourceNotFoundError(name)
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses []pexec.ManagedProcess
}

// FilterFromConfig given a config this function will remove elements from the manager and return removed resources in a new manager.
func (manager *resourceManager) FilterFromConfig(ctx context.Context, conf *config.Config, logger golog.Logger) (*resourceManager, error) {
	filtered := newResourceManager(manager.opts, logger)
	var allErrs error
	for _, conf := range conf.Processes {
		if manager.opts.untrustedEnv {
			allErrs = multierr.Combine(allErrs, errProcessesDisabled)
			break
		}

		proc, ok := manager.processManager.RemoveProcessByID(conf.ID)
		if !ok {
			manager.logger.Errorw("couldn't remove process", "process", conf.ID)
			continue
		}
		if _, err := filtered.processManager.AddProcess(ctx, proc, false); err != nil {
			manager.logger.Errorw("couldn't add process", "process", conf.ID, "error", err)
		}
	}

	for _, conf := range conf.Remotes {
		remoteName := fromRemoteNameToRemoteNodeName(conf.Name)
		if _, ok := manager.resources.Node(remoteName); !ok {
			continue
		}
		if _, ok := filtered.resources.Node(remoteName); ok {
			continue
		}
		subG, err := manager.resources.SubGraphFrom(remoteName)
		if err != nil {
			manager.logger.Errorw("error while getting a subgraph", "error", err)
		}
		err = filtered.resources.MergeAdd(subG)
		if err != nil {
			manager.logger.Errorw("error doing a merge addition", "error", err)
		}
	}
	for _, compConf := range conf.Components {
		rName := compConf.ResourceName()
		if _, ok := manager.resources.Node(rName); !ok {
			continue
		}
		if _, ok := filtered.resources.Node(rName); ok {
			continue
		}
		subG, err := manager.resources.SubGraphFrom(rName)
		if err != nil {
			manager.logger.Errorw("error while getting a subgraph", "error", err)
		}
		err = filtered.resources.MergeAdd(subG)
		if err != nil {
			manager.logger.Errorw("error doing a merge addition ", "error", err)
		}
	}
	for _, conf := range conf.Services {
		rName := conf.ResourceName()

		// Disable shell service when in untrusted env
		if manager.opts.untrustedEnv && rName.Subtype == shell.Subtype {
			allErrs = multierr.Combine(allErrs, errShellServiceDisabled)
			continue
		}

		if _, ok := manager.resources.Node(rName); !ok {
			continue
		}
		if _, ok := filtered.resources.Node(rName); ok {
			continue
		}
		subG, err := manager.resources.SubGraphFrom(rName)
		if err != nil {
			manager.logger.Errorw("error while getting a subgraph", "error", err)
		}
		err = filtered.resources.MergeAdd(subG)
		if err != nil {
			manager.logger.Errorw("error doing a merge addition", "error", err)
		}
	}
	manager.resources.MergeRemove(filtered.resources)
	return filtered, allErrs
}

func remoteDialOptions(config config.Remote, opts resourceManagerOptions) []rpc.DialOption {
	var dialOpts []rpc.DialOption
	if opts.debug {
		dialOpts = append(dialOpts, rpc.WithDialDebug())
	}
	if config.Insecure {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}
	if opts.allowInsecureCreds {
		dialOpts = append(dialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
	}
	if opts.tlsConfig != nil {
		dialOpts = append(dialOpts, rpc.WithTLSConfig(opts.tlsConfig))
	}
	if config.Auth.Credentials != nil {
		if config.Auth.Entity == "" {
			dialOpts = append(dialOpts, rpc.WithCredentials(*config.Auth.Credentials))
		} else {
			dialOpts = append(dialOpts, rpc.WithEntityCredentials(config.Auth.Entity, *config.Auth.Credentials))
		}
	} else {
		// explicitly unset credentials so they are not fed to remotes unintentionally.
		dialOpts = append(dialOpts, rpc.WithEntityCredentials("", rpc.Credentials{}))
	}

	if config.Auth.ExternalAuthAddress != "" {
		dialOpts = append(dialOpts, rpc.WithExternalAuth(
			config.Auth.ExternalAuthAddress,
			config.Auth.ExternalAuthToEntity,
		))
	}

	if config.Auth.ExternalAuthInsecure {
		dialOpts = append(dialOpts, rpc.WithExternalAuthInsecure())
	}

	if config.Auth.SignalingServerAddress != "" {
		wrtcOpts := rpc.DialWebRTCOptions{
			Config:                 &rpc.DefaultWebRTCConfiguration,
			SignalingServerAddress: config.Auth.SignalingServerAddress,
			SignalingAuthEntity:    config.Auth.SignalingAuthEntity,
		}
		if config.Auth.SignalingCreds != nil {
			wrtcOpts.SignalingCreds = *config.Auth.SignalingCreds
		}
		dialOpts = append(dialOpts, rpc.WithWebRTCOptions(wrtcOpts))

		if config.Auth.Managed {
			// managed robots use TLS authN/Z
			dialOpts = append(dialOpts, rpc.WithDialMulticastDNSOptions(rpc.DialMulticastDNSOptions{
				RemoveAuthCredentials: true,
			}))
		}
	}
	return dialOpts
}
