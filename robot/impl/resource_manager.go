package robotimpl

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module/modmanager"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	modif "go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

func init() {
	if err := cleanAppImageEnv(); err != nil {
		logging.Global().Errorw("error cleaning up app image environement", "error", err)
	}
}

var (
	errShellServiceDisabled = errors.New("shell service disabled in an untrusted environment")
	errProcessesDisabled    = errors.New("processes disabled in an untrusted environment")
)

// resourceManager manages the actual parts that make up a robot.
type resourceManager struct {
	resources      *resource.Graph
	processManager pexec.ProcessManager
	processConfigs map[string]pexec.ProcessConfig
	moduleManager  modif.ModuleManager
	opts           resourceManagerOptions
	logger         logging.Logger
	configLock     sync.Mutex
}

type resourceManagerOptions struct {
	debug              bool
	fromCommand        bool
	allowInsecureCreds bool
	untrustedEnv       bool
	tlsConfig          *tls.Config
}

// newResourceManager returns a properly initialized set of parts.
// moduleManager will not be initialized until startModuleManager is called.
func newResourceManager(
	opts resourceManagerOptions,
	logger logging.Logger,
) *resourceManager {
	return &resourceManager{
		resources:      resource.NewGraph(),
		processManager: newProcessManager(opts, logger),
		processConfigs: make(map[string]pexec.ProcessConfig),
		opts:           opts,
		logger:         logger,
	}
}

func newProcessManager(
	opts resourceManagerOptions,
	logger logging.Logger,
) pexec.ProcessManager {
	if opts.untrustedEnv {
		return pexec.NoopProcessManager
	}
	return pexec.NewProcessManager(logger.AsZap())
}

func fromRemoteNameToRemoteNodeName(name string) resource.Name {
	return resource.NewName(client.RemoteAPI, name)
}

func (manager *resourceManager) startModuleManager(
	parentAddr string,
	removeOrphanedResources func(context.Context, []resource.Name),
	untrustedEnv bool,
	viamHomeDir string,
	robotCloudID string,
	logger logging.Logger,
) {
	mmOpts := modmanageroptions.Options{
		UntrustedEnv:            untrustedEnv,
		RemoveOrphanedResources: removeOrphanedResources,
		ViamHomeDir:             viamHomeDir,
		RobotCloudID:            robotCloudID,
	}
	manager.moduleManager = modmanager.NewManager(parentAddr, logger, mmOpts)
}

// addRemote adds a remote to the manager.
func (manager *resourceManager) addRemote(
	ctx context.Context,
	rr internalRemoteRobot,
	gNode *resource.GraphNode,
	c config.Remote,
) {
	rName := fromRemoteNameToRemoteNodeName(c.Name)
	if gNode == nil {
		gNode = resource.NewConfiguredGraphNode(resource.Config{
			ConvertedAttributes: &c,
		}, rr, builtinModel)
		if err := manager.resources.AddNode(rName, gNode); err != nil {
			manager.logger.Errorw("failed to add new node for remote", "name", rName, "error", err)
			return
		}
	} else {
		gNode.SwapResource(rr, builtinModel)
	}
	manager.updateRemoteResourceNames(ctx, rName, rr)
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

var (
	unknownModel = resource.DefaultModelFamily.WithModel("unknown")
	builtinModel = resource.DefaultModelFamily.WithModel("builtin")
)

// maybe in the future this can become an actual resource with its own type
// so that one can depend on robots/remotes interchangeably.
type internalRemoteRobot interface {
	resource.Resource
	robot.Robot
}

// updateRemoteResourceNames is called when the Remote robot has changed (either connection or disconnection).
// It will pull the current remote resources and update the resource tree adding or removing nodes accordingly
// If any local resources are dependent on a remote resource two things can happen
//  1. The remote resource already is in the tree and nothing will happen.
//  2. A remote resource is being deleted but a local resource depends on it; it will be removed
//     and its local children will be destroyed.
func (manager *resourceManager) updateRemoteResourceNames(
	ctx context.Context,
	remoteName resource.Name,
	rr internalRemoteRobot,
) bool {
	activeResourceNames := map[resource.Name]bool{}
	newResources := rr.ResourceNames()
	oldResources := manager.remoteResourceNames(remoteName)
	for _, res := range oldResources {
		activeResourceNames[res] = false
	}

	anythingChanged := false

	for _, resName := range newResources {
		remoteResName := resName
		res, err := rr.ResourceByName(remoteResName) // this returns a remote known OR foreign resource client
		if err != nil {
			if errors.Is(err, client.ErrMissingClientRegistration) {
				manager.logger.CDebugw(ctx, "couldn't obtain remote resource interface",
					"name", remoteResName,
					"reason", err)
			} else {
				manager.logger.Errorw("couldn't obtain remote resource interface",
					"name", remoteResName,
					"reason", err)
			}
			continue
		}

		resName = resName.PrependRemote(remoteName.Name)
		gNode, ok := manager.resources.Node(resName)

		if _, alreadyCurrent := activeResourceNames[resName]; alreadyCurrent {
			activeResourceNames[resName] = true
			if ok && !gNode.IsUninitialized() {
				continue
			}
		}

		if ok {
			gNode.SwapResource(res, unknownModel)
		} else {
			gNode = resource.NewConfiguredGraphNode(resource.Config{}, res, unknownModel)
			if err := manager.resources.AddNode(resName, gNode); err != nil {
				manager.logger.Errorw("failed to add remote resource node", "name", resName, "error", err)
			}
		}

		err = manager.resources.AddChild(resName, remoteName)
		if err != nil {
			manager.logger.Errorw(
				"error while trying add node as a dependency of remote",
				"node", resName,
				"remote", remoteName)
		} else {
			anythingChanged = true
		}
	}

	for resName, isActive := range activeResourceNames {
		if isActive {
			continue
		}
		manager.logger.CDebugw(ctx, "removing remote resource", "name", resName)
		if err := manager.markChildrenForUpdate(resName); err != nil {
			manager.logger.Errorw(
				"failed to mark children of remote for update",
				"resource", resName,
				"reason", err)
			continue
		}
		gNode, ok := manager.resources.Node(resName)
		if !ok {
			manager.logger.Errorw(
				"failed to find remote node for closure",
				"resource", resName)
			continue
		}
		if err := gNode.Close(ctx); err != nil {
			manager.logger.Errorw(
				"failed to close remote node",
				"resource", resName,
				"reason", err)
		}
		anythingChanged = true
	}
	return anythingChanged
}

func (manager *resourceManager) updateRemotesResourceNames(ctx context.Context) bool {
	anythingChanged := false
	for _, name := range manager.resources.Names() {
		gNode, _ := manager.resources.Node(name)
		if name.API == client.RemoteAPI {
			res, err := gNode.Resource()
			if err == nil {
				if rr, ok := res.(internalRemoteRobot); ok {
					anythingChanged = anythingChanged || manager.updateRemoteResourceNames(ctx, name, rr)
				}
			}
		}
	}
	return anythingChanged
}

// RemoteNames returns the names of all remotes in the manager.
func (manager *resourceManager) RemoteNames() []string {
	names := []string{}
	for _, k := range manager.resources.Names() {
		res, _ := manager.resources.Node(k)
		if k.API == client.RemoteAPI && res != nil {
			names = append(names, k.Name)
		}
	}
	return names
}

func (manager *resourceManager) anyResourcesNotConfigured() bool {
	for _, name := range manager.resources.Names() {
		res, ok := manager.resources.Node(name)
		if !ok {
			continue
		}
		if res.NeedsReconfigure() {
			return true
		}
	}
	return false
}

func (manager *resourceManager) internalResourceNames() []resource.Name {
	names := []resource.Name{}
	for _, k := range manager.resources.Names() {
		if k.API.Type.Namespace != resource.APINamespaceRDKInternal {
			continue
		}
		names = append(names, k)
	}
	return names
}

// ResourceNames returns the names of all resources in the manager.
func (manager *resourceManager) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for _, k := range manager.resources.Names() {
		if k.API == client.RemoteAPI ||
			k.API.Type.Namespace == resource.APINamespaceRDKInternal {
			continue
		}
		gNode, ok := manager.resources.Node(k)
		if !ok || !gNode.HasResource() {
			continue
		}
		names = append(names, k)
	}
	return names
}

// ResourceRPCAPIs returns the types of all resource RPC APIs in use by the manager.
func (manager *resourceManager) ResourceRPCAPIs() []resource.RPCAPI {
	resourceAPIs := resource.RegisteredAPIs()

	types := map[resource.API]*desc.ServiceDescriptor{}
	for _, k := range manager.resources.Names() {
		if k.API.Type.Namespace == resource.APINamespaceRDKInternal {
			continue
		}
		if k.API == client.RemoteAPI {
			gNode, ok := manager.resources.Node(k)
			if !ok || !gNode.HasResource() {
				continue
			}
			res, err := gNode.Resource()
			if err != nil {
				manager.logger.Errorw(
					"error getting remote from node",
					"remote",
					k.Name,
					"err",
					err,
				)
				continue
			}
			rr, ok := res.(robot.Robot)
			if !ok {
				manager.logger.Debugw(
					"remote does not implement robot interface",
					"remote",
					k.Name,
					"type",
					reflect.TypeOf(res),
				)
				continue
			}
			manager.mergeResourceRPCAPIsWithRemote(rr, types)
			continue
		}
		if k.ContainsRemoteNames() {
			continue
		}
		if types[k.API] != nil {
			continue
		}

		st, ok := resourceAPIs[k.API]
		if !ok {
			continue
		}

		if st.ReflectRPCServiceDesc != nil {
			types[k.API] = st.ReflectRPCServiceDesc
		}
	}
	typesList := make([]resource.RPCAPI, 0, len(types))
	for k, v := range types {
		typesList = append(typesList, resource.RPCAPI{
			API:  k,
			Desc: v,
		})
	}
	return typesList
}

// mergeResourceRPCAPIsWithRemotes merges types from the manager itself as well as its
// remotes.
func (manager *resourceManager) mergeResourceRPCAPIsWithRemote(r robot.Robot, types map[resource.API]*desc.ServiceDescriptor) {
	remoteTypes := r.ResourceRPCAPIs()
	for _, remoteType := range remoteTypes {
		if svcName, ok := types[remoteType.API]; ok {
			if svcName.GetFullyQualifiedName() != remoteType.Desc.GetFullyQualifiedName() {
				manager.logger.Errorw(
					"remote proto service name clashes with another of the same API",
					"existing", svcName.GetFullyQualifiedName(),
					"remote", remoteType.Desc.GetFullyQualifiedName())
			}
			continue
		}
		types[remoteType.API] = remoteType.Desc
	}
}

func (manager *resourceManager) closeResource(ctx context.Context, res resource.Resource) error {
	allErrs := res.Close(ctx)

	resName := res.Name()
	if manager.moduleManager != nil && manager.moduleManager.IsModularResource(resName) {
		if err := manager.moduleManager.RemoveResource(ctx, resName); err != nil {
			allErrs = multierr.Combine(err, errors.Wrap(err, "error removing modular resource for closure"))
		}
	}

	return allErrs
}

// removeMarkedAndClose removes all resources marked for removal from the graph and
// also closes them. It accepts an excludeFromClose in case some marked resources were
// already removed (e.g. renamed resources that count as remove + add but need to close
// before add) or need to be removed in a different way (e.g. web internal service last).
func (manager *resourceManager) removeMarkedAndClose(
	ctx context.Context,
	excludeFromClose map[resource.Name]struct{},
) error {
	var allErrs error
	toClose := manager.resources.RemoveMarked()
	for _, res := range toClose {
		resName := res.Name()
		if _, ok := excludeFromClose[resName]; ok {
			continue
		}
		allErrs = multierr.Combine(allErrs, manager.closeResource(ctx, res))
	}
	return allErrs
}

// Close attempts to close/stop all parts.
func (manager *resourceManager) Close(ctx context.Context) error {
	manager.resources.MarkForRemoval(manager.resources.Clone())

	var allErrs error
	if err := manager.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error stopping process manager"))
	}

	// our caller will close web
	excludeWebFromClose := map[resource.Name]struct{}{
		web.InternalServiceName: {},
	}
	if err := manager.removeMarkedAndClose(ctx, excludeWebFromClose); err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}

	// moduleManager may be nil in tests, and must be closed last, after resources within have been closed properly above
	if manager.moduleManager != nil {
		if err := manager.moduleManager.Close(ctx); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing module manager"))
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

	// first handle remotes since they may reveal unresolved dependencies
	for _, resName := range manager.resources.FindNodesByAPI(client.RemoteAPI) {
		gNode, ok := manager.resources.Node(resName)
		if !ok || !gNode.NeedsReconfigure() {
			continue
		}
		var verb string
		if gNode.IsUninitialized() {
			verb = "configuring"
		} else {
			verb = "reconfiguring"
		}
		manager.logger.CDebugw(ctx, fmt.Sprintf("now %s a remote", verb), "resource", resName)
		switch resName.API {
		case client.RemoteAPI:
			remConf, err := resource.NativeConfig[*config.Remote](gNode.Config())
			if err != nil {
				manager.logger.Errorw(
					"remote config error",
					"error",
					err,
				)
				continue
			}
			// this is done in config validation but partial start rules require us to check again
			if _, err := remConf.Validate(""); err != nil {
				gNode.LogAndSetLastError(
					fmt.Errorf("remote config validation error: %w", err), "remote", remConf.Name)
				continue
			}
			rr, err := manager.processRemote(ctx, *remConf)
			if err != nil {
				gNode.LogAndSetLastError(
					fmt.Errorf("error connecting to remote: %w", err), "remote", remConf.Name)
				continue
			}
			manager.addRemote(ctx, rr, gNode, *remConf)
			rr.SetParentNotifier(func() {
				if robot.closeContext.Err() != nil {
					return
				}

				// Trigger completeConfig goroutine execution when a change in remote
				// is detected.
				select {
				case <-robot.closeContext.Done():
					return
				case robot.triggerConfig <- struct{}{}:
				}
			})
		default:
			err := errors.New("config is not a remote config")
			manager.logger.Errorw(err.Error(), "resource", resName)
		}
	}

	// now resolve prior to sorting in case there's anything newly discovered
	if err := manager.resources.ResolveDependencies(manager.logger); err != nil {
		// debug here since the resolver will log on its own
		manager.logger.CDebugw(ctx, "error resolving dependencies", "error", err)
	}

	resourceNames := manager.resources.ReverseTopologicalSort()
	timeout := rutils.GetResourceConfigurationTimeout(manager.logger)
	for _, resName := range resourceNames {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resChan := make(chan struct{}, 1)
		resName := resName
		ctxWithTimeout, timeoutCancel := context.WithTimeout(ctx, timeout)
		defer timeoutCancel()
		robot.reconfigureWorkers.Add(1)

		goutils.PanicCapturingGo(func() {
			defer func() {
				resChan <- struct{}{}
				robot.reconfigureWorkers.Done()
			}()
			gNode, ok := manager.resources.Node(resName)
			if !ok || !gNode.NeedsReconfigure() {
				return
			}
			if !(resName.API.IsComponent() || resName.API.IsService()) {
				return
			}
			var verb string
			if gNode.IsUninitialized() {
				verb = "configuring"
			} else {
				verb = "reconfiguring"
			}
			manager.logger.CDebugw(ctx, fmt.Sprintf("now %s resource", verb), "resource", resName)
			conf := gNode.Config()

			// this is done in config validation but partial start rules require us to check again
			if _, err := conf.Validate("", resName.API.Type.Name); err != nil {
				gNode.LogAndSetLastError(
					fmt.Errorf("resource config validation error: %w", err),
					"resource", conf.ResourceName(),
					"model", conf.Model)
				return
			}
			if manager.moduleManager.Provides(conf) {
				if _, err := manager.moduleManager.ValidateConfig(ctxWithTimeout, conf); err != nil {
					gNode.LogAndSetLastError(
						fmt.Errorf("modular resource config validation error: %w", err),
						"resource", conf.ResourceName(),
						"model", conf.Model)
					return
				}
			}

			switch {
			case resName.API.IsComponent(), resName.API.IsService():
				newRes, newlyBuilt, err := manager.processResource(ctxWithTimeout, conf, gNode, robot)
				if newlyBuilt || err != nil {
					if err := manager.markChildrenForUpdate(resName); err != nil {
						manager.logger.Errorw(
							"failed to mark children of resource for update",
							"resource", resName,
							"reason", err)
					}
				}

				if err != nil {
					gNode.LogAndSetLastError(
						fmt.Errorf("resource build error: %w", err),
						"resource", conf.ResourceName(),
						"model", conf.Model)
					return
				}

				// if the ctxWithTimeout fails with DeadlineExceeded, then that means that
				// resource generation is running async, and we don't currently have good
				// validation around how this might affect the resource graph. So, we avoid
				// updating the graph to be safe.
				if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
					manager.logger.Errorw("error building resource", "resource", conf.ResourceName(), "model", conf.Model, "error", ctxWithTimeout.Err())
				} else {
					gNode.SwapResource(newRes, conf.Model)
				}

			default:
				err := errors.New("config is not for a component or service")
				gNode.LogAndSetLastError(err, "resource", resName)
			}
		})

		select {
		case <-resChan:
		case <-ctxWithTimeout.Done():
			if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
				robot.logger.CWarn(ctx, resource.NewBuildTimeoutError(resName))
			}
		case <-ctx.Done():
			return
		}
	} // for-each resource name
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
func (manager *resourceManager) processRemote(
	ctx context.Context,
	config config.Remote,
) (*client.RobotClient, error) {
	dialOpts := remoteDialOptions(config, manager.opts)
	manager.logger.CDebugw(ctx, "connecting now to remote", "remote", config.Name)
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
	manager.logger.CDebugw(ctx, "connected now to remote", "remote", config.Name)
	return robotClient, nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (manager *resourceManager) RemoteByName(name string) (internalRemoteRobot, bool) {
	rName := resource.NewName(client.RemoteAPI, name)
	if gNode, ok := manager.resources.Node(rName); ok {
		remRes, err := gNode.Resource()
		if err != nil {
			manager.logger.Errorw("error getting remote", "remote", name, "err", err)
			return nil, false
		}
		remRobot, ok := remRes.(internalRemoteRobot)
		if !ok {
			manager.logger.Errorw("tried to access remote but its not a robot interface", "remote", name, "type", reflect.TypeOf(remRes))
		}
		return remRobot, ok
	}
	return nil, false
}

func (manager *resourceManager) markChildrenForUpdate(rName resource.Name) error {
	sg, err := manager.resources.SubGraphFrom(rName)
	if err != nil {
		return err
	}
	sorted := sg.TopologicalSort()
	for _, name := range sorted {
		if name == rName || name.ContainsRemoteNames() {
			continue // ignore self and non-local resources
		}
		gNode, ok := manager.resources.Node(name)
		if !ok {
			continue
		}

		gNode.SetNeedsUpdate()
	}
	return nil
}

func (manager *resourceManager) processResource(
	ctx context.Context,
	conf resource.Config,
	gNode *resource.GraphNode,
	r *localRobot,
) (resource.Resource, bool, error) {
	if gNode.IsUninitialized() {
		newRes, err := r.newResource(ctx, gNode, conf)
		if err != nil {
			return nil, false, err
		}
		return newRes, true, nil
	}

	currentRes, err := gNode.UnsafeResource()
	if err != nil {
		return nil, false, err
	}

	resName := conf.ResourceName()
	deps, err := r.getDependencies(ctx, resName, gNode)
	if err != nil {
		return nil, false, multierr.Combine(err, manager.closeResource(ctx, currentRes))
	}

	isModular := manager.moduleManager.Provides(conf)
	if gNode.ResourceModel() == conf.Model {
		if isModular {
			if err := manager.moduleManager.ReconfigureResource(ctx, conf, modmanager.DepsToNames(deps)); err != nil {
				return nil, false, err
			}
			return currentRes, false, nil
		}

		gNode.SetLogLevel(conf.LogConfiguration.Level)
		err = currentRes.Reconfigure(ctx, deps, conf)
		if err == nil {
			return currentRes, false, nil
		}

		if !resource.IsMustRebuildError(err) {
			return nil, false, err
		}
	} else {
		manager.logger.CDebugw(ctx, "resource models differ so it must be rebuilt",
			"name", resName, "old_model", gNode.ResourceModel(), "new_model", conf.Model)
	}

	manager.logger.CDebugw(ctx, "rebuilding", "name", resName)
	if err := r.manager.closeResource(ctx, currentRes); err != nil {
		manager.logger.CError(ctx, err)
	}
	newRes, err := r.newResource(ctx, gNode, conf)
	if err != nil {
		gNode.UnsetResource()
		manager.logger.CDebugw(ctx,
			"failed to build resource of new model, removing closed resource of old model from graph node",
			"name", resName,
			"old_model", gNode.ResourceModel(),
			"new_model", conf.Model,
		)
		return nil, false, err
	}
	return newRes, true, nil
}

// markResourceForUpdate marks the given resource in the graph to be updated. If it does not exist, a new node
// is inserted. If it does exist, it's properly marked. Once this is done, all information needed to build/reconfigure
// will be available when we call completeConfig.
func (manager *resourceManager) markResourceForUpdate(name resource.Name, conf resource.Config, deps []string) error {
	gNode, hasNode := manager.resources.Node(name)
	if hasNode {
		gNode.SetNewConfig(conf, deps)
		// reset parentage
		for _, parent := range manager.resources.GetAllParentsOf(name) {
			manager.resources.RemoveChild(name, parent)
		}
		return nil
	}
	gNode = resource.NewUnconfiguredGraphNode(conf, deps)
	if err := manager.resources.AddNode(name, gNode); err != nil {
		return errors.Errorf("failed to add new node for unconfigured resource %q: %v", name, err)
	}
	return nil
}

// updateResources will use the difference between the current config
// and next one to create resource nodes with configs that completeConfig will later on use.
// Ideally at the end of this function we should have a complete graph representation of the configuration
// for all well known resources. For resources that cannot be matched up to their dependencies, they will
// be in an unresolved state for later resolution.
func (manager *resourceManager) updateResources(
	ctx context.Context,
	conf *config.Diff,
) error {
	manager.configLock.Lock()
	defer manager.configLock.Unlock()
	var allErrs error

	// modules are not added into the resource tree as they belong to the module manager
	for _, mod := range conf.Added.Modules {
		// this is done in config validation but partial start rules require us to check again
		if err := mod.Validate(""); err != nil {
			manager.logger.Errorw("module config validation error; skipping", "module", mod.Name, "error", err)
			continue
		}
		if err := manager.moduleManager.Add(ctx, mod); err != nil {
			manager.logger.Errorw("error adding module", "module", mod.Name, "error", err)
			continue
		}
	}

	for _, mod := range conf.Modified.Modules {
		// this is done in config validation but partial start rules require us to check again
		if err := mod.Validate(""); err != nil {
			manager.logger.Errorw("module config validation error; skipping", "module", mod.Name, "error", err)
			continue
		}
		orphanedResourceNames, err := manager.moduleManager.Reconfigure(ctx, mod)
		if err != nil {
			manager.logger.Errorw("error reconfiguring module", "module", mod.Name, "error", err)
		}
		for _, resToClose := range manager.markResourcesRemoved(orphanedResourceNames, nil) {
			if err := resToClose.Close(ctx); err != nil {
				manager.logger.Errorw("error closing now orphaned resource", "resource",
					resToClose.Name().String(), "module", mod.Name, "error", err)
			}
		}
	}

	if manager.moduleManager != nil {
		if err := manager.moduleManager.ResolveImplicitDependenciesInConfig(ctx, conf); err != nil {
			manager.logger.Errorw("error adding implicit dependencies", "error", err)
		}
	}

	for _, s := range conf.Added.Services {
		rName := s.ResourceName()
		if manager.opts.untrustedEnv && rName.API == shell.API {
			allErrs = multierr.Combine(allErrs, errShellServiceDisabled)
			continue
		}
		allErrs = multierr.Combine(allErrs, manager.markResourceForUpdate(rName, s, s.Dependencies()))
	}
	for _, c := range conf.Added.Components {
		rName := c.ResourceName()
		allErrs = multierr.Combine(allErrs, manager.markResourceForUpdate(rName, c, c.Dependencies()))
	}
	for _, r := range conf.Added.Remotes {
		rName := fromRemoteNameToRemoteNodeName(r.Name)
		rCopy := r
		allErrs = multierr.Combine(allErrs, manager.markResourceForUpdate(rName, resource.Config{ConvertedAttributes: &rCopy}, []string{}))
	}
	for _, c := range conf.Modified.Components {
		rName := c.ResourceName()
		allErrs = multierr.Combine(allErrs, manager.markResourceForUpdate(rName, c, c.Dependencies()))
	}
	for _, s := range conf.Modified.Services {
		rName := s.ResourceName()

		// Disable shell service when in untrusted env
		if manager.opts.untrustedEnv && rName.API == shell.API {
			allErrs = multierr.Combine(allErrs, errShellServiceDisabled)
			continue
		}

		allErrs = multierr.Combine(allErrs, manager.markResourceForUpdate(rName, s, s.Dependencies()))
	}
	for _, r := range conf.Modified.Remotes {
		rName := fromRemoteNameToRemoteNodeName(r.Name)
		rCopy := r
		allErrs = multierr.Combine(allErrs, manager.markResourceForUpdate(rName, resource.Config{ConvertedAttributes: &rCopy}, []string{}))
	}

	// processes are not added into the resource tree as they belong to a process manager
	for _, p := range conf.Added.Processes {
		if manager.opts.untrustedEnv {
			allErrs = multierr.Combine(allErrs, errProcessesDisabled)
			break
		}

		// this is done in config validation but partial start rules require us to check again
		if err := p.Validate(""); err != nil {
			manager.logger.Errorw("process config validation error; skipping", "process", p.Name, "error", err)
			continue
		}

		_, err := manager.processManager.AddProcessFromConfig(ctx, p)
		if err != nil {
			manager.logger.Errorw("error while adding process; skipping", "process", p.ID, "error", err)
			continue
		}
		manager.processConfigs[p.ID] = p
	}
	for _, p := range conf.Modified.Processes {
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

		// Remove processConfig from map in case re-addition fails.
		delete(manager.processConfigs, p.ID)

		// this is done in config validation but partial start rules require us to check again
		if err := p.Validate(""); err != nil {
			manager.logger.Errorw("process config validation error; skipping", "process", p.Name, "error", err)
			continue
		}

		_, err := manager.processManager.AddProcessFromConfig(ctx, p)
		if err != nil {
			manager.logger.Errorw("error while changing process; skipping", "process", p.ID, "error", err)
			continue
		}
		manager.processConfigs[p.ID] = p
	}

	return allErrs
}

// ResourceByName returns the given resource by fully qualified name, if it exists;
// returns an error otherwise.
func (manager *resourceManager) ResourceByName(name resource.Name) (resource.Resource, error) {
	if gNode, ok := manager.resources.Node(name); ok {
		res, err := gNode.Resource()
		if err != nil {
			return nil, resource.NewNotAvailableError(name, err)
		}
		return res, nil
	}
	// if we haven't found a resource of this name then we are going to look into remote resources to find it.
	// This is kind of weird and arguably you could have a ResourcesByPartialName that would match against
	// a string and not a resource name (e.g. expressions).
	if !name.ContainsRemoteNames() {
		keys := manager.resources.FindNodesByShortNameAndAPI(name)
		if len(keys) > 1 {
			return nil, rutils.NewRemoteResourceClashError(name.Name)
		}
		if len(keys) == 1 {
			gNode, ok := manager.resources.Node(keys[0])
			if ok {
				res, err := gNode.Resource()
				if err != nil {
					return nil, resource.NewNotAvailableError(name, err)
				}
				return res, nil
			}
		}
	}
	return nil, resource.NewNotFoundError(name)
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses []pexec.ManagedProcess
}

// markRemoved marks all resources in the config (assumed to be a removed diff) for removal. This must be called
// before updateResources. After updateResources is called, any resources still marked will be fully removed from
// the graph and closed.
func (manager *resourceManager) markRemoved(
	ctx context.Context,
	conf *config.Config,
	logger logging.Logger,
) (pexec.ProcessManager, []resource.Resource, map[resource.Name]struct{}) {
	processesToClose := newProcessManager(manager.opts, logger)
	for _, conf := range conf.Processes {
		if manager.opts.untrustedEnv {
			continue
		}

		proc, ok := manager.processManager.RemoveProcessByID(conf.ID)
		if !ok {
			manager.logger.Errorw("couldn't remove process", "process", conf.ID)
			continue
		}
		delete(manager.processConfigs, conf.ID)
		if _, err := processesToClose.AddProcess(ctx, proc, false); err != nil {
			manager.logger.Errorw("couldn't add process", "process", conf.ID, "error", err)
		}
	}

	var resourcesToMark []resource.Name
	for _, conf := range conf.Modules {
		orphanedResourceNames, err := manager.moduleManager.Remove(conf.Name)
		if err != nil {
			manager.logger.Errorw("error removing module", "module", conf.Name, "error", err)
		}
		resourcesToMark = append(resourcesToMark, orphanedResourceNames...)
	}

	for _, conf := range conf.Remotes {
		resourcesToMark = append(
			resourcesToMark,
			fromRemoteNameToRemoteNodeName(conf.Name),
		)
	}
	for _, conf := range append(conf.Components, conf.Services...) {
		resourcesToMark = append(resourcesToMark, conf.ResourceName())
	}
	markedResourceNames := map[resource.Name]struct{}{}
	addNames := func(names ...resource.Name) {
		for _, name := range names {
			markedResourceNames[name] = struct{}{}
		}
	}
	resourcesToCloseBeforeComplete := manager.markResourcesRemoved(resourcesToMark, addNames)
	return processesToClose, resourcesToCloseBeforeComplete, markedResourceNames
}

// markResourcesRemoved marks all passed in resources (assumed to be resource
// names of components, services or remotes) for removal.
func (manager *resourceManager) markResourcesRemoved(
	rNames []resource.Name,
	addNames func(names ...resource.Name),
) []resource.Resource {
	var resourcesToCloseBeforeComplete []resource.Resource
	for _, rName := range rNames {
		// Disable changes to shell in untrusted
		if manager.opts.untrustedEnv && rName.API == shell.API {
			continue
		}

		resNode, ok := manager.resources.Node(rName)
		if !ok {
			continue
		}
		resourcesToCloseBeforeComplete = append(resourcesToCloseBeforeComplete,
			resource.NewCloseOnlyResource(rName, resNode.Close))
		subG, err := manager.resources.SubGraphFrom(rName)
		if err != nil {
			manager.logger.Errorw("error while getting a subgraph", "error", err)
			continue
		}
		if addNames != nil {
			addNames(subG.Names()...)
		}
		manager.resources.MarkForRemoval(subG)
	}
	return resourcesToCloseBeforeComplete
}

// createConfig will create a config.Config based on the current state of the
// resource graph, processManager and moduleManager. The created config will
// possibly contain default services registered by the RDK and not specified by
// the user in their config.
func (manager *resourceManager) createConfig() *config.Config {
	conf := &config.Config{}

	for _, resName := range manager.resources.Names() {
		// Ignore non-local resources.
		if resName.ContainsRemoteNames() {
			continue
		}
		gNode, ok := manager.resources.Node(resName)
		if !ok {
			continue
		}
		resConf := gNode.Config()

		// gocritic will complain that this if-else chain should be a switch, but
		// it's really a mix of == and bool method checks.
		//
		//nolint: gocritic
		if resName.API == client.RemoteAPI {
			remoteConf, err := rutils.AssertType[*config.Remote](resConf.ConvertedAttributes)
			if err != nil {
				manager.logger.Errorw("error getting remote config",
					"remote", resName.String(),
					"error", err)
				continue
			}

			conf.Remotes = append(conf.Remotes, *remoteConf)
		} else if resName.API.IsComponent() {
			conf.Components = append(conf.Components, resConf)
		} else if resName.API.IsService() &&
			resName.API.Type.Namespace != resource.APINamespaceRDKInternal {
			conf.Services = append(conf.Services, resConf)
		}
	}

	conf.Modules = append(conf.Modules, manager.moduleManager.Configs()...)
	for _, processConf := range manager.processConfigs {
		conf.Processes = append(conf.Processes, processConf)
	}

	return conf
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
