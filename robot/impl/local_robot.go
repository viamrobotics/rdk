// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/app/packages/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/utils"
)

var _ = robot.LocalRobot(&localRobot{})

// localRobot satisfies robot.LocalRobot and defers most
// logic to its manager.
type localRobot struct {
	mu      sync.Mutex
	manager *resourceManager
	config  *config.Config

	operations                 *operation.Manager
	sessionManager             session.Manager
	packageManager             packages.ManagerSyncer
	cloudConnSvc               cloud.ConnectionService
	logger                     golog.Logger
	activeBackgroundWorkers    sync.WaitGroup
	cancelBackgroundWorkers    func()
	closeContext               context.Context
	triggerConfig              chan struct{}
	configTicker               *time.Ticker
	revealSensitiveConfigDiffs bool

	lastWeakDependentsRound int64

	// internal services that are in the graph but we also hold onto
	webSvc   web.Service
	frameSvc framesystem.Service
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *localRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.manager.RemoteByName(name)
}

// ResourceByName returns a resource by name. If it does not exist
// nil is returned.
func (r *localRobot) ResourceByName(name resource.Name) (resource.Resource, error) {
	return r.manager.ResourceByName(name)
}

// RemoteNames returns the names of all known remote robots.
func (r *localRobot) RemoteNames() []string {
	return r.manager.RemoteNames()
}

// ResourceNames returns the names of all known resources.
func (r *localRobot) ResourceNames() []resource.Name {
	return r.manager.ResourceNames()
}

// ResourceRPCAPIs returns all known resource RPC APIs in use.
func (r *localRobot) ResourceRPCAPIs() []resource.RPCAPI {
	return r.manager.ResourceRPCAPIs()
}

// ProcessManager returns the process manager for the robot.
func (r *localRobot) ProcessManager() pexec.ProcessManager {
	return r.manager.processManager
}

// OperationManager returns the operation manager for the robot.
func (r *localRobot) OperationManager() *operation.Manager {
	return r.operations
}

// SessionManager returns the session manager for the robot.
func (r *localRobot) SessionManager() session.Manager {
	return r.sessionManager
}

// PackageManager returns the package manager for the robot.
func (r *localRobot) PackageManager() packages.Manager {
	return r.packageManager
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *localRobot) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// we will stop and close web ourselves since modules need it to be
	// removed properly and in the right order, so grab it before its removed
	// from the graph/closed automatically.
	if r.webSvc != nil {
		// we may not have the web service if we closed prematurely
		r.webSvc.Stop()
	}
	if r.cancelBackgroundWorkers != nil {
		r.cancelBackgroundWorkers()
		r.cancelBackgroundWorkers = nil
		if r.configTicker != nil {
			r.configTicker.Stop()
		}
		close(r.triggerConfig)
	}
	r.activeBackgroundWorkers.Wait()

	var err error
	if r.cloudConnSvc != nil {
		err = multierr.Combine(err, r.cloudConnSvc.Close(ctx))
	}
	if r.manager != nil {
		err = multierr.Combine(err, r.manager.Close(ctx))
	}
	if r.packageManager != nil {
		err = multierr.Combine(err, r.packageManager.Close(ctx))
	}
	if r.webSvc != nil {
		err = multierr.Combine(err, r.webSvc.Close(ctx))
	}
	r.sessionManager.Close()
	return err
}

// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement.
func (r *localRobot) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	// Stop all operations
	for _, op := range r.OperationManager().All() {
		op.Cancel()
	}

	// Stop all stoppable resources
	resourceErrs := []string{}
	for _, name := range r.ResourceNames() {
		res, err := r.ResourceByName(name)
		if err != nil {
			resourceErrs = append(resourceErrs, name.Name)
			continue
		}

		if actuator, ok := res.(resource.Actuator); ok {
			if err := actuator.Stop(ctx, extra[name]); err != nil {
				resourceErrs = append(resourceErrs, name.Name)
			}
		}
	}

	if len(resourceErrs) > 0 {
		return errors.Errorf("failed to stop components named %s", strings.Join(resourceErrs, ","))
	}
	return nil
}

// Config returns the config used to construct the robot. Only local resources are returned.
// This is allowed to be partial or empty.
func (r *localRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgCpy := *r.config
	cfgCpy.Components = append([]resource.Config{}, cfgCpy.Components...)

	return &cfgCpy, nil
}

// Logger returns the logger the robot is using.
func (r *localRobot) Logger() golog.Logger {
	return r.logger
}

// StartWeb starts the web server, will return an error if server is already up.
func (r *localRobot) StartWeb(ctx context.Context, o weboptions.Options) (err error) {
	return r.webSvc.Start(ctx, o)
}

// StopWeb stops the web server, will be a noop if server is not up.
func (r *localRobot) StopWeb(ctx context.Context) error {
	return r.webSvc.Close(ctx)
}

// WebAddress return the web service's address.
func (r *localRobot) WebAddress() (string, error) {
	return r.webSvc.Address(), nil
}

// ModuleAddress return the module service's address.
func (r *localRobot) ModuleAddress() (string, error) {
	return r.webSvc.ModuleAddress(), nil
}

// RemoveOrphanedResources is called by the module manager when a module has
// changed (either due to crash or successful restart) and some or all of the
// resources it handled have been orphaned. It will remove the orphaned
// resources from the resource tree and Close them.
func (r *localRobot) RemoveOrphanedResources(ctx context.Context,
	orphanedResourceNames []resource.Name,
) {
	for _, resName := range orphanedResourceNames {
		r.manager.logger.Debugw("removing orphaned resource", "name", resName)
		subG, err := r.manager.resources.SubGraphFrom(resName)
		if err != nil {
			r.manager.logger.Errorw("error while getting a subgraph", "error", err)
			continue
		}
		r.manager.resources.MarkForRemoval(subG)
	}

	r.manager.completeConfig(ctx, r)
	r.updateWeakDependents(ctx)

	// removeMarkedAndClose should Close all marked resources.
	removedNames, removedErr := r.manager.removeMarkedAndClose(ctx, nil)
	if removedErr != nil {
		r.manager.logger.Errorw("error while removing and closing nodes", "error",
			removedErr)
		return
	}
	for _, removedName := range removedNames {
		// Remove dependents of removed resources from r.config; leaving these
		// resources in the stored config means they cannot be correctly re-added
		// when their dependency reappears.
		//
		// TODO(RSDK-2876): remove this code when we start referring to a config
		// generated from resource graph instead of r.config.
		for i, c := range r.config.Components {
			if c.ResourceName() == removedName {
				r.config.Components[i] = r.config.Components[len(r.config.Components)-1]
				r.config.Components = r.config.Components[:len(r.config.Components)-1]
			}
		}
		for i, s := range r.config.Services {
			if s.ResourceName() == removedName {
				r.config.Services[i] = r.config.Services[len(r.config.Services)-1]
				r.config.Services = r.config.Services[:len(r.config.Services)-1]
			}
		}
	}
}

// remoteNameByResource returns the remote the resource is pulled from, if found.
// False can mean either the resource doesn't exist or is local to the robot.
func remoteNameByResource(resourceName resource.Name) (string, bool) {
	if !resourceName.ContainsRemoteNames() {
		return "", false
	}
	remote := strings.Split(resourceName.Remote, ":")
	return remote[0], true
}

func (r *localRobot) Status(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	r.mu.Lock()
	resources := make(map[resource.Name]resource.Resource, len(r.manager.resources.Names()))
	for _, name := range r.ResourceNames() {
		res, err := r.ResourceByName(name)
		if err != nil {
			r.mu.Unlock()
			return nil, resource.NewNotFoundError(name)
		}
		resources[name] = res
	}
	r.mu.Unlock()

	namesToDedupe := resourceNames
	// if no names, return all
	if len(namesToDedupe) == 0 {
		namesToDedupe = make([]resource.Name, 0, len(resources))
		for name := range resources {
			namesToDedupe = append(namesToDedupe, name)
		}
	}

	// dedupe resourceNames
	deduped := make(map[resource.Name]struct{}, len(namesToDedupe))
	for _, name := range namesToDedupe {
		deduped[name] = struct{}{}
	}

	// group each resource name by remote and also get its corresponding name on the remote
	groupedResources := make(map[string]map[resource.Name]resource.Name)
	for name := range deduped {
		remoteName, ok := remoteNameByResource(name)
		if !ok {
			continue
		}
		mappings, ok := groupedResources[remoteName]
		if !ok {
			mappings = make(map[resource.Name]resource.Name)
		}
		mappings[name.PopRemote()] = name
		groupedResources[remoteName] = mappings
	}
	// make requests and map it back to the local resource name
	remoteStatuses := make(map[resource.Name]robot.Status)
	for remoteName, resourceNamesMappings := range groupedResources {
		remote, ok := r.RemoteByName(remoteName)
		if !ok {
			// should never happen
			r.Logger().Errorw("remote robot not found while creating status", "remote", remoteName)
			continue
		}
		remoteRNames := make([]resource.Name, 0, len(resourceNamesMappings))
		for n := range resourceNamesMappings {
			remoteRNames = append(remoteRNames, n)
		}

		s, err := remote.Status(ctx, remoteRNames)
		if err != nil {
			return nil, err
		}
		for _, stat := range s {
			mappedName, ok := resourceNamesMappings[stat.Name]
			if !ok {
				// should never happen
				r.Logger().Errorw(
					"failed to find corresponding resource name for remote resource name while creating status",
					"resource", stat.Name,
				)
				continue
			}
			stat.Name = mappedName
			remoteStatuses[mappedName] = stat
		}
	}
	statuses := make([]robot.Status, 0, len(deduped))
	for name := range deduped {
		resourceStatus, ok := remoteStatuses[name]
		if !ok {
			res, ok := resources[name]
			if !ok {
				return nil, resource.NewNotFoundError(name)
			}
			// if resource API has an associated CreateStatus method, use that
			// otherwise return an empty status
			var status interface{} = map[string]interface{}{}
			var err error
			if apiReg, ok := resource.LookupGenericAPIRegistration(name.API); ok && apiReg.Status != nil {
				status, err = apiReg.Status(ctx, res)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get status from %q", name)
				}
			}
			resourceStatus = robot.Status{Name: name, Status: status}
		}
		statuses = append(statuses, resourceStatus)
	}
	return statuses, nil
}

func newWithResources(
	ctx context.Context,
	cfg *config.Config,
	resources map[resource.Name]resource.Resource,
	logger golog.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	var rOpts options
	for _, opt := range opts {
		opt.apply(&rOpts)
	}

	closeCtx, cancel := context.WithCancel(ctx)
	r := &localRobot{
		manager: newResourceManager(
			resourceManagerOptions{
				debug:              cfg.Debug,
				fromCommand:        cfg.FromCommand,
				allowInsecureCreds: cfg.AllowInsecureCreds,
				untrustedEnv:       cfg.UntrustedEnv,
				tlsConfig:          cfg.Network.TLSConfig,
			},
			logger,
		),
		operations:                 operation.NewManager(logger),
		logger:                     logger,
		closeContext:               closeCtx,
		cancelBackgroundWorkers:    cancel,
		triggerConfig:              make(chan struct{}),
		configTicker:               nil,
		revealSensitiveConfigDiffs: rOpts.revealSensitiveConfigDiffs,
		cloudConnSvc:               cloud.NewCloudConnectionService(cfg.Cloud, logger),
	}
	var heartbeatWindow time.Duration
	if cfg.Network.Sessions.HeartbeatWindow == 0 {
		heartbeatWindow = config.DefaultSessionHeartbeatWindow
	} else {
		heartbeatWindow = cfg.Network.Sessions.HeartbeatWindow
	}
	r.sessionManager = robot.NewSessionManager(r, heartbeatWindow)

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(context.Background()); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()

	if cfg.Cloud != nil && cfg.Cloud.AppAddress != "" {
		_, cloudConn, err := r.cloudConnSvc.AcquireConnection(ctx)
		if err == nil {
			r.packageManager, err = packages.NewCloudManager(pb.NewPackageServiceClient(cloudConn), cfg.PackagePath, logger)
			if err != nil {
				return nil, err
			}
		} else {
			if !errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			r.logger.Debug("Using no-op PackageManager when internet not available")
			r.packageManager = packages.NewNoopManager()
		}
	} else {
		r.logger.Debug("Using no-op PackageManager when Cloud config is not available")
		r.packageManager = packages.NewNoopManager()
	}

	// start process manager early
	if err := r.manager.processManager.Start(ctx); err != nil {
		return nil, err
	}

	// we assume these never appear in our configs and as such will not be removed from the
	// resource graph
	r.webSvc = web.New(r, logger, rOpts.webOptions...)
	r.frameSvc = framesystem.New(ctx, r, logger)
	if err := r.manager.resources.AddNode(
		web.InternalServiceName,
		resource.NewConfiguredGraphNode(resource.Config{}, r.webSvc, builtinModel)); err != nil {
		return nil, err
	}
	if err := r.manager.resources.AddNode(
		framesystem.InternalServiceName,
		resource.NewConfiguredGraphNode(resource.Config{}, r.frameSvc, builtinModel)); err != nil {
		return nil, err
	}
	if err := r.manager.resources.AddNode(
		r.packageManager.Name(),
		resource.NewConfiguredGraphNode(resource.Config{}, r.packageManager, builtinModel)); err != nil {
		return nil, err
	}
	if err := r.manager.resources.AddNode(
		r.cloudConnSvc.Name(),
		resource.NewConfiguredGraphNode(resource.Config{}, r.cloudConnSvc, builtinModel)); err != nil {
		return nil, err
	}

	if err := r.webSvc.StartModule(ctx); err != nil {
		return nil, err
	}

	// Once web service is started, start module manager and add initially
	// specified modules.
	r.manager.startModuleManager(r.webSvc.ModuleAddress(), cfg.UntrustedEnv, logger)
	for _, mod := range cfg.Modules {
		err := r.manager.moduleManager.Add(ctx, mod)
		if err != nil {
			return nil, err
		}
	}

	r.activeBackgroundWorkers.Add(1)
	r.configTicker = time.NewTicker(5 * time.Second)
	// This goroutine tries to complete the config and update weak dependencies
	// if any resources are not configured. It executes every 5 seconds or when
	// manually triggered. Manual triggers are sent when changes in remotes are
	// detected and in testing.
	goutils.ManagedGo(func() {
		for {
			select {
			case <-closeCtx.Done():
				return
			case <-r.configTicker.C:
			case <-r.triggerConfig:
			}
			anyChanges := r.manager.updateRemotesResourceNames(closeCtx)
			if r.manager.anyResourcesNotConfigured() {
				anyChanges = true
				r.manager.completeConfig(closeCtx, r)
			}
			if anyChanges {
				r.updateWeakDependents(ctx)
			}
		}
	}, r.activeBackgroundWorkers.Done)

	r.config = &config.Config{}
	r.Reconfigure(ctx, cfg)

	for name, res := range resources {
		if err := r.manager.resources.AddNode(
			name, resource.NewConfiguredGraphNode(resource.Config{}, res, unknownModel)); err != nil {
			return nil, err
		}
	}

	if len(resources) != 0 {
		r.updateWeakDependents(ctx)
	}

	successful = true
	return r, nil
}

// New returns a new robot with parts sourced from the given config.
func New(
	ctx context.Context,
	cfg *config.Config,
	logger golog.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	return newWithResources(ctx, cfg, nil, logger, opts...)
}

// getDependencies derives a collection of dependencies from a robot for a given
// component's name. We don't use the resource manager for this information since
// it is not be constructed at this point.
func (r *localRobot) getDependencies(
	ctx context.Context,
	rName resource.Name,
	gNode *resource.GraphNode,
) (resource.Dependencies, error) {
	if deps := gNode.UnresolvedDependencies(); len(deps) != 0 {
		return nil, errors.Errorf("resource has unresolved dependencies: %v", deps)
	}
	allDeps := make(resource.Dependencies)
	var needUpdate bool
	for _, dep := range r.manager.resources.GetAllParentsOf(rName) {
		if node, ok := r.manager.resources.Node(dep); ok {
			if r.lastWeakDependentsRound <= node.UpdatedAt() {
				needUpdate = true
			}
		}
		// Specifically call ResourceByName and not directly to the manager since this
		// will only return fully configured and available resources (not marked for removal
		// and no last error).
		r, err := r.ResourceByName(dep)
		if err != nil {
			return nil, &resource.DependencyNotReadyError{Name: dep.Name, Reason: err}
		}
		allDeps[dep] = r
	}
	nodeConf := gNode.Config()
	for weakDepName, weakDepRes := range r.getWeakDependencies(rName, nodeConf.API, nodeConf.Model) {
		if _, ok := allDeps[weakDepName]; ok {
			continue
		}
		allDeps[weakDepName] = weakDepRes
	}

	if needUpdate {
		r.updateWeakDependents(ctx)
	}

	return allDeps, nil
}

func (r *localRobot) getWeakDependencyMatchers(api resource.API, model resource.Model) []internal.ResourceMatcher {
	reg, ok := resource.LookupRegistration(api, model)
	if !ok {
		return nil
	}
	return reg.WeakDependencies
}

func (r *localRobot) getWeakDependencies(resName resource.Name, api resource.API, model resource.Model) resource.Dependencies {
	weakDepMatchers := r.getWeakDependencyMatchers(api, model)

	allResources := map[resource.Name]resource.Resource{}
	internalResources := map[resource.Name]resource.Resource{}
	components := map[resource.Name]resource.Resource{}
	slamServices := map[resource.Name]resource.Resource{}
	for _, n := range r.manager.resources.Names() {
		if !(n.API.IsComponent() || n.API.IsService()) {
			continue
		}
		res, err := r.ResourceByName(n)
		if err != nil {
			if !resource.IsDependencyNotReadyError(err) && !resource.IsNotAvailableError(err) {
				r.Logger().Debugw("error finding resource while getting weak dependencies", "resource", n, "error", err)
			}
			continue
		}
		allResources[n] = res
		switch {
		case n.API.IsComponent():
			components[n] = res
		case n.API.SubtypeName == slam.API.SubtypeName:
			slamServices[n] = res
		case n.API.Type.Namespace == resource.APINamespaceRDKInternal:
			internalResources[n] = res
		}
	}

	deps := make(resource.Dependencies, len(weakDepMatchers))
	for _, matcher := range weakDepMatchers {
		match := func(resouces map[resource.Name]resource.Resource) {
			for k, v := range resouces {
				if k == resName {
					continue
				}
				deps[k] = v
			}
		}
		switch matcher {
		case internal.ComponentDependencyWildcardMatcher:
			match(components)
		case internal.SLAMDependencyWildcardMatcher:
			match(slamServices)
		default:
			// no other matchers supported right now. you could imagine a LiteralMatcher in the future
		}
	}
	return deps
}

func (r *localRobot) newResource(
	ctx context.Context,
	gNode *resource.GraphNode,
	conf resource.Config,
) (res resource.Resource, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(errors.Errorf("%v", r), "panic creating resource")
		}
	}()
	resName := conf.ResourceName()
	resInfo, ok := resource.LookupRegistration(resName.API, conf.Model)
	if !ok {
		return nil, errors.Errorf("unknown resource type: %s and/or model: %s", resName.API, conf.Model)
	}

	deps, err := r.getDependencies(ctx, resName, gNode)
	if err != nil {
		return nil, err
	}

	c, ok := resource.LookupGenericAPIRegistration(resName.API)
	if ok {
		// If MaxInstance equals zero then there is not a limit on the number of resources
		if c.MaxInstance != 0 {
			if err := r.checkMaxInstance(resName.API, c.MaxInstance); err != nil {
				return nil, err
			}
		}
	}

	resLogger := r.logger.Named(conf.ResourceName().String())
	if resInfo.Constructor != nil {
		return resInfo.Constructor(ctx, deps, conf, resLogger)
	}
	if resInfo.DeprecatedRobotConstructor == nil {
		return nil, errors.Errorf("invariant: no constructor for %q", conf.API)
	}
	r.logger.Warnw("using deprecated robot constructor", "api", resName.API, "model", conf.Model)
	return resInfo.DeprecatedRobotConstructor(ctx, r, conf, resLogger)
}

func (r *localRobot) updateWeakDependents(ctx context.Context) {
	// track that we are current in resources up to the latest update time. This will
	// be used to determine if this method should be called while completing a config.
	r.lastWeakDependentsRound = r.manager.resources.LastUpdatedTime()

	allResources := map[resource.Name]resource.Resource{}
	internalResources := map[resource.Name]resource.Resource{}
	components := map[resource.Name]resource.Resource{}
	for _, n := range r.manager.resources.Names() {
		if !(n.API.IsComponent() || n.API.IsService()) {
			continue
		}
		res, err := r.ResourceByName(n)
		if err != nil {
			if !resource.IsDependencyNotReadyError(err) && !resource.IsNotAvailableError(err) {
				r.Logger().Debugw("error finding resource during weak dependent update", "resource", n, "error", err)
			}
			continue
		}
		allResources[n] = res
		switch {
		case n.API.IsComponent():
			components[n] = res
		case n.API.Type.Namespace == resource.APINamespaceRDKInternal:
			internalResources[n] = res
		}
	}

	// NOTE(erd): this is intentionally hard coded since these services are treated specially with
	// how they request dependencies or consume the robot's config. We should make an effort to
	// formalize these as servcices that while internal, obey the reconfigure lifecycle.
	// For example, the framesystem should depend on all input enabled components while the web
	// service depends on all resources.
	// For now, we pass all resources and empty configs.
	for resName, res := range internalResources {
		switch resName {
		case web.InternalServiceName, framesystem.InternalServiceName:
			if err := res.Reconfigure(ctx, allResources, resource.Config{}); err != nil {
				r.Logger().Errorw("failed to reconfigure internal service", "service", resName, "error", err)
			}
		case packages.InternalServiceName, cloud.InternalServiceName:
		default:
			r.logger.Warnw("do not know how to reconfigure internal service", "service", resName)
		}
	}

	updateResourceWeakDependents := func(conf resource.Config) {
		resName := conf.ResourceName()
		resNode, ok := r.manager.resources.Node(resName)
		if !ok {
			return
		}
		res, err := resNode.Resource()
		if err != nil {
			return
		}
		if len(r.getWeakDependencyMatchers(conf.API, conf.Model)) == 0 {
			return
		}
		deps, err := r.getDependencies(ctx, resName, resNode)
		if err != nil {
			r.Logger().Errorw("failed to get dependencies during weak update; skipping", "resource", resName, "error", err)
			return
		}
		if err := res.Reconfigure(ctx, deps, conf); err != nil {
			r.Logger().Errorw("failed to reconfigure resource with weak dependencies", "resource", resName, "error", err)
		}
	}
	for _, conf := range r.config.Components {
		updateResourceWeakDependents(conf)
	}
	for _, conf := range r.config.Services {
		updateResourceWeakDependents(conf)
	}
}

// FrameSystemConfig returns the info of each individual part that makes up a robot's frame system.
func (r *localRobot) FrameSystemConfig(
	ctx context.Context,
	additionalTransforms []*referenceframe.LinkInFrame,
) (framesystemparts.Parts, error) {
	return r.frameSvc.Config(ctx, additionalTransforms)
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (r *localRobot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	return r.frameSvc.TransformPose(ctx, pose, dst, additionalTransforms)
}

// TransformPointCloud will transform the pointcloud to the desired frame in the robot's frame system.
// Do not move the robot between the generation of the initial pointcloud and the receipt
// of the transformed pointcloud because that will make the transformations inaccurate.
func (r *localRobot) TransformPointCloud(
	ctx context.Context,
	srcpc pointcloud.PointCloud,
	srcName, dstName string,
) (pointcloud.PointCloud, error) {
	return r.frameSvc.TransformPointCloud(ctx, srcpc, srcName, dstName)
}

// RobotFromConfigPath is a helper to read and process a config given its path and then create a robot based on it.
func RobotFromConfigPath(ctx context.Context, cfgPath string, logger golog.Logger, opts ...Option) (robot.LocalRobot, error) {
	cfg, err := config.Read(ctx, cfgPath, logger)
	if err != nil {
		logger.Error("cannot read config")
		return nil, err
	}
	return RobotFromConfig(ctx, cfg, logger, opts...)
}

// RobotFromConfig is a helper to process a config and then create a robot based on it.
func RobotFromConfig(ctx context.Context, cfg *config.Config, logger golog.Logger, opts ...Option) (robot.LocalRobot, error) {
	tlsConfig := config.NewTLSConfig(cfg)
	processedCfg, err := config.ProcessConfig(cfg, tlsConfig)
	if err != nil {
		return nil, err
	}
	return New(ctx, processedCfg, logger, opts...)
}

// RobotFromResources creates a new robot consisting of the given resources. Using RobotFromConfig is preferred
// to support more streamlined reconfiguration functionality.
func RobotFromResources(
	ctx context.Context,
	resources map[resource.Name]resource.Resource,
	logger golog.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	return newWithResources(ctx, &config.Config{}, resources, logger, opts...)
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (r *localRobot) DiscoverComponents(ctx context.Context, qs []resource.DiscoveryQuery) ([]resource.Discovery, error) {
	// dedupe queries
	deduped := make(map[resource.DiscoveryQuery]struct{}, len(qs))
	for _, q := range qs {
		deduped[q] = struct{}{}
	}

	discoveries := make([]resource.Discovery, 0, len(deduped))
	for q := range deduped {
		reg, ok := resource.LookupRegistration(q.API, q.Model)
		if !ok || reg.Discover == nil {
			r.logger.Warnw("no discovery function registered", "api", q.API, "model", q.Model)
			continue
		}

		if reg.Discover != nil {
			discovered, err := reg.Discover(ctx, r.logger.Named("discovery"))
			if err != nil {
				return nil, &resource.DiscoverError{Query: q}
			}
			discoveries = append(discoveries, resource.Discovery{Query: q, Results: discovered})
		}
	}
	return discoveries, nil
}

func dialRobotClient(
	ctx context.Context,
	config config.Remote,
	logger golog.Logger,
	dialOpts ...rpc.DialOption,
) (*client.RobotClient, error) {
	rOpts := []client.RobotClientOption{client.WithDialOptions(dialOpts...), client.WithRemoteName(config.Name)}

	if config.ConnectionCheckInterval != 0 {
		rOpts = append(rOpts, client.WithCheckConnectedEvery(config.ConnectionCheckInterval))
	}
	if config.ReconnectInterval != 0 {
		rOpts = append(rOpts, client.WithReconnectEvery(config.ReconnectInterval))
	}

	robotClient, err := client.New(
		ctx,
		config.Address,
		logger,
		rOpts...,
	)
	if err != nil {
		return nil, err
	}
	return robotClient, nil
}

// Reconfigure will safely reconfigure a robot based on the given config. It will make
// a best effort to remove no longer in use parts, but if it fails to do so, they could
// possibly leak resources.
// The given config is assumed to be owned by the robot now.
func (r *localRobot) Reconfigure(ctx context.Context, newConfig *config.Config) {
	var allErrs error

	// Add default services and process their dependencies. Dependencies may
	// already come from config validation so we check that here.
	seen := make(map[resource.API]int)
	for idx, val := range newConfig.Services {
		seen[val.API] = idx
	}
	for _, name := range resource.DefaultServices() {
		existingConfIdx, hasExistingConf := seen[name.API]
		var svcCfg resource.Config
		if hasExistingConf {
			svcCfg = newConfig.Services[existingConfIdx]
		} else {
			svcCfg = resource.Config{
				Name:  name.Name,
				Model: resource.DefaultServiceModel,
				API:   name.API,
			}
		}

		if svcCfg.ConvertedAttributes != nil || svcCfg.Attributes != nil {
			// previously processed
			continue
		}

		// we find dependencies through configs, so we must try to validate even a default config
		if reg, ok := resource.LookupRegistration(svcCfg.API, svcCfg.Model); ok && reg.AttributeMapConverter != nil {
			converted, err := reg.AttributeMapConverter(utils.AttributeMap{})
			if err != nil {
				allErrs = multierr.Combine(allErrs, errors.Wrapf(err, "error converting attributes for %s", svcCfg.API))
				continue
			}
			svcCfg.ConvertedAttributes = converted
			deps, err := converted.Validate("")
			if err != nil {
				allErrs = multierr.Combine(allErrs, errors.Wrapf(err, "error getting default service dependencies for %s", svcCfg.API))
				continue
			}
			svcCfg.ImplicitDependsOn = deps
		}
		if hasExistingConf {
			newConfig.Services[existingConfIdx] = svcCfg
		} else {
			newConfig.Services = append(newConfig.Services, svcCfg)
		}
	}

	// Sync Packages before reconfiguring rest of robot and resolving references to any packages
	// in the config.
	// TODO(RSDK-1849): Make this non-blocking so other resources that do not require packages can run before package sync finishes.
	// TODO(RSDK-2710) this should really use Reconfigure for the package and should allow itself to check
	// if anything has changed.
	err := r.packageManager.Sync(ctx, newConfig.Packages)
	if err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}

	err = r.replacePackageReferencesWithPaths(newConfig)
	if err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}

	// Now that we have the new config and all references are resolved, diff it with the old
	// config to see what has changed.
	diff, err := config.DiffConfigs(*r.config, *newConfig, r.revealSensitiveConfigDiffs)
	if err != nil {
		r.logger.Errorw("error diffing the configs", "error", err)
		return
	}
	if diff.ResourcesEqual {
		return
	}

	// If something was added or modified, go through components and services in
	// diff.Added and diff.Modified, call Validate on all those that are modularized,
	// and store implicit dependencies.
	validateModularResources := func(confs []resource.Config) {
		for i, c := range confs {
			if r.manager.moduleManager.Provides(c) {
				implicitDeps, err := r.manager.moduleManager.ValidateConfig(ctx, c)
				if err != nil {
					r.logger.Errorw("modular config validation error found in resource: "+c.Name, "error", err)
					continue
				}

				// Modify resource to add its implicit dependencies.
				confs[i].ImplicitDependsOn = implicitDeps
			}
		}
	}
	if diff.Added != nil {
		validateModularResources(diff.Added.Components)
		validateModularResources(diff.Added.Services)
	}
	if diff.Modified != nil {
		validateModularResources(diff.Modified.Components)
		validateModularResources(diff.Modified.Services)
	}

	if r.revealSensitiveConfigDiffs {
		r.logger.Debugf("(re)configuring with %+v", diff)
	}

	// First we remove resources and their children that are not in the graph.
	processesToClose, resourcesToCloseBeforeComplete, _ := r.manager.markRemoved(ctx, diff.Removed, r.logger)

	// Second we update the resource graph.
	allErrs = multierr.Combine(allErrs, r.manager.updateResources(ctx, diff))
	r.config = newConfig

	allErrs = multierr.Combine(allErrs, processesToClose.Stop())

	// Third we attempt to complete the config (see function for details)
	alreadyClosed := make(map[resource.Name]struct{}, len(resourcesToCloseBeforeComplete))
	for _, res := range resourcesToCloseBeforeComplete {
		allErrs = multierr.Combine(allErrs, r.manager.closeResource(ctx, res))
		// avoid a double close later
		alreadyClosed[res.Name()] = struct{}{}
	}
	r.manager.completeConfig(ctx, r)
	r.updateWeakDependents(ctx)

	removedNames, removedErr := r.manager.removeMarkedAndClose(ctx, alreadyClosed)
	if removedErr != nil {
		allErrs = multierr.Combine(allErrs, removedErr)
	}
	for _, removedName := range removedNames {
		// Remove dependents of removed resources from r.config; leaving these
		// resources in the stored config means they cannot be correctly re-added
		// when their dependency reappears.
		//
		// TODO(RSDK-2876): remove this code when we start referring to a config
		// generated from resource graph instead of r.config.
		for i, c := range r.config.Components {
			if c.ResourceName() == removedName {
				r.config.Components[i] = r.config.Components[len(r.config.Components)-1]
				r.config.Components = r.config.Components[:len(r.config.Components)-1]
			}
		}
		for i, s := range r.config.Services {
			if s.ResourceName() == removedName {
				r.config.Services[i] = r.config.Services[len(r.config.Services)-1]
				r.config.Services = r.config.Services[:len(r.config.Services)-1]
			}
		}
	}

	// cleanup unused packages after all old resources have been closed above. This ensures
	// processes are shutdown before any files are deleted they are using.
	allErrs = multierr.Combine(allErrs, r.packageManager.Cleanup(ctx))

	if allErrs != nil {
		r.logger.Errorw("the following errors were gathered during reconfiguration", "errors", allErrs)
	}
}

func walkConvertedAttributes[T any](pacMan packages.ManagerSyncer, convertedAttributes T, allErrs error) (T, error) {
	// Replace all package references with the actual path containing the package
	// on the robot.
	var asIfc interface{} = convertedAttributes
	if walker, ok := asIfc.(utils.Walker); ok {
		newAttrs, err := walker.Walk(packages.NewPackagePathVisitor(pacMan))
		if err != nil {
			allErrs = multierr.Combine(allErrs, err)
			return convertedAttributes, allErrs
		}
		newAttrsTyped, err := utils.AssertType[T](newAttrs)
		if err != nil {
			var zero T
			return zero, err
		}
		convertedAttributes = newAttrsTyped
	}
	return convertedAttributes, allErrs
}

func (r *localRobot) replacePackageReferencesWithPaths(cfg *config.Config) error {
	var allErrs error
	for i, s := range cfg.Services {
		s.ConvertedAttributes, allErrs = walkConvertedAttributes(r.packageManager, s.ConvertedAttributes, allErrs)
		cfg.Services[i] = s
	}

	for i, c := range cfg.Components {
		c.ConvertedAttributes, allErrs = walkConvertedAttributes(r.packageManager, c.ConvertedAttributes, allErrs)
		cfg.Components[i] = c
	}

	return allErrs
}

// checkMaxInstance checks to see if the local robot has reached the maximum number of a specific resource type that are local.
func (r *localRobot) checkMaxInstance(api resource.API, max int) error {
	maxInstance := 0
	for _, n := range r.ResourceNames() {
		if n.API == api && !n.ContainsRemoteNames() {
			maxInstance++
			if maxInstance == max {
				return errors.Errorf("max instance number reached for resource type: %s", api)
			}
		}
	}
	return nil
}
