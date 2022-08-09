package robotimpl

import (
	"context"
	"crypto/tls"
	"os"
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
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rutils "go.viam.com/rdk/utils"
)

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
}

type resourceManagerOptions struct {
	debug              bool
	fromCommand        bool
	allowInsecureCreds bool
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
	return &resourceManager{
		resources:      resource.NewGraph(),
		processManager: pexec.NewProcessManager(logger),
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
func (manager *resourceManager) updateRemoteResourceNames(ctx context.Context, remoteName resource.Name, rr robot.Robot, lr *localRobot) {
	visited := map[resource.Name]bool{}
	newResources := rr.ResourceNames()
	oldResources := manager.remoteResourceNames(remoteName)
	for _, res := range oldResources {
		visited[res] = false
	}

	for _, res := range newResources {
		rrName := res
		res = res.PrependRemote(resource.RemoteName(remoteName.Name))
		if _, ok := visited[res]; ok {
			visited[res] = true
			continue
		}
		iface, err := rr.ResourceByName(rrName) // this returns the remote object client OR a foreign resource
		if err != nil {
			manager.logger.Errorw("couldn't obtain remote resource interface",
				"name", rrName,
				"reason", err)
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
		}
	}
	for res, visit := range visited {
		// this loops through all of visited to see which ones are FALSE, meaning they are
		// NOT a part of the new resources in current robot. If remote is disconnected
		// then none of the nodes in this graph will have been visited
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
		}
	}
}

func (manager *resourceManager) updateRemotesResourceNames(ctx context.Context, r *localRobot) {
	for _, name := range manager.resources.Names() {
		iface, _ := manager.resources.Node(name)
		if name.ResourceType == remoteTypeName {
			if rr, ok := iface.(robot.Robot); ok {
				manager.updateRemoteResourceNames(ctx, name, rr, r)
			}
		}
	}
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
				manager.logger.Errorw("remote robot doesn't implement the robot interface",
					"remote", k,
					"type", iface)
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

		types[k.Subtype] = st.ReflectRPCServiceDesc
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
func (manager *resourceManager) Close(ctx context.Context) error {
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
		manager.logger.Infow("we are now handling the resource ", "resource", r.Name)
		if c, ok := wrap.config.(config.Component); ok {
			iface, err := manager.processComponent(ctx, r, c, wrap.real, robot)
			if err != nil {
				manager.logger.Errorw("error building component", "error", err)
				continue
			}
			manager.resources.AddNode(r, iface)
		} else if s, ok := wrap.config.(config.Service); ok {
			iface, err := manager.processService(ctx, s, wrap.real, robot)
			if err != nil {
				manager.logger.Errorw("error building service", "error", err)
				continue
			}
			manager.resources.AddNode(r, iface)
		} else if rc, ok := wrap.config.(config.Remote); ok {
			rr, err := manager.processRemote(ctx, rc)
			if err != nil {
				manager.logger.Errorw("error connecting to remote", "error", err)
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
		return nil, errors.Wrapf(err, "couldn't connect to robot remote (%s)", config.Address)
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
			manager.logger.Errorw("tried to access remote but its not a robot interface", "remote_name", name, "type", iface)
		}
		return part, ok
	}
	return nil, false
}

func (manager *resourceManager) reconfigureResource(ctx context.Context, old, newR interface{}) (interface{}, error) {
	if old == nil {
		// if the oldPart was never created, replace directly with the new resource
		return newR, nil
	}

	oldPart, oldResourceIsReconfigurable := old.(resource.Reconfigurable)
	newPart, newResourceIsReconfigurable := newR.(resource.Reconfigurable)

	switch {
	case oldResourceIsReconfigurable != newResourceIsReconfigurable:
		// this is an indicator of a serious constructor problem
		// for the resource subtype.
		reconfError := errors.Errorf(
			"new type %T is reconfigurable whereas old type %T is not",
			newR, old)
		if oldResourceIsReconfigurable {
			reconfError = errors.Errorf(
				"old type %T is reconfigurable whereas new type %T is not",
				old, newR)
		}
		return nil, reconfError
	case oldResourceIsReconfigurable && newResourceIsReconfigurable:
		// if we are dealing with a reconfigurable resource
		// use the new resource to reconfigure the old one.
		if err := oldPart.Reconfigure(ctx, newPart); err != nil {
			return nil, err
		}
		return old, nil
	case !oldResourceIsReconfigurable && !newResourceIsReconfigurable:
		// if we are not dealing with a reconfigurable resource
		// we want to close the old resource and replace it with the
		// new.
		if err := utils.TryClose(ctx, old); err != nil {
			return nil, err
		}
		return newR, nil
	default:
		return nil, errors.Errorf("unexpected outcome during reconfiguration of type %T and type %T",
			old, newR)
	}
}

func (manager *resourceManager) processService(ctx context.Context,
	c config.Service,
	old interface{},
	robot *localRobot,
) (interface{}, error) {
	if old == nil {
		return robot.newService(ctx, c)
	}
	svc, err := robot.newService(ctx, c)
	if err != nil {
		return nil, err
	}
	return manager.reconfigureResource(ctx, old, svc)
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
	obj, canValidate := old.(config.CompononentUpdate)
	res := config.Rebuild
	if canValidate {
		res = obj.UpdateAction(&conf)
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
		rr, err := manager.reconfigureResource(ctx, old, nr)
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

// updateResourceGraph using the difference between current config
// and next we create resource wrappers to be consumed ny completeConfig later on
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
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, s, []string{}, fn))
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
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, s, []string{}, fn))
	}
	for _, r := range config.Modified.Remotes {
		rName := fromRemoteNameToRemoteNodeName(r.Name)
		allErrs = multierr.Combine(allErrs, manager.wrapResource(rName, r, []string{}, fn))
	}
	// processes are not added into the resource tree as they belong to a process manager
	if err := cleanAppImageEnv(); err != nil {
		manager.logger.Errorw("error cleaning up app image environement", "error", err)
	}
	for _, p := range config.Added.Processes {
		_, err := manager.processManager.AddProcessFromConfig(ctx, p)
		if err != nil {
			manager.logger.Errorw("error while adding process, skipping", "process", p.ID, "error", err)
			continue
		}
	}
	for _, p := range config.Modified.Processes {
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
		if _, ok = robotPart.(*resourcePlaceholder); !ok {
			return robotPart, nil
		}
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

	for _, conf := range conf.Processes {
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
	return filtered, nil
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
