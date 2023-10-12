// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
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
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/utils"
)

var _ = robot.LocalRobot(&localRobot{})

// localRobot satisfies robot.LocalRobot and defers most
// logic to its manager.
type localRobot struct {
	mu            sync.Mutex
	manager       *resourceManager
	mostRecentCfg config.Config

	operations                 *operation.Manager
	sessionManager             session.Manager
	packageManager             packages.ManagerSyncer
	cloudConnSvc               cloud.ConnectionService
	logger                     golog.Logger
	activeBackgroundWorkers    sync.WaitGroup
	reconfigureWorkers         sync.WaitGroup
	cancelBackgroundWorkers    func()
	closeContext               context.Context
	triggerConfig              chan struct{}
	configTicker               *time.Ticker
	revealSensitiveConfigDiffs bool

	lastWeakDependentsRound atomic.Int64

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
	}
	r.activeBackgroundWorkers.Wait()
	r.sessionManager.Close()

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

// Config returns a config representing the current state of the robot.
func (r *localRobot) Config() *config.Config {
	cfg := r.mostRecentCfg

	// Use resource manager to generate Modules, Remotes, Components, Processes
	// and Services.
	//
	// NOTE(benji): it would be great if the resource manager could somehow
	// generate Cloud, Packages, Network and Auth fields.
	generatedCfg := r.manager.createConfig()
	cfg.Modules = generatedCfg.Modules
	cfg.Remotes = generatedCfg.Remotes
	cfg.Components = generatedCfg.Components
	cfg.Processes = generatedCfg.Processes
	cfg.Services = generatedCfg.Services

	return &cfg
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
func (r *localRobot) StopWeb() {
	r.webSvc.Stop()
}

// WebAddress return the web service's address.
func (r *localRobot) WebAddress() (string, error) {
	return r.webSvc.Address(), nil
}

// ModuleAddress return the module service's address.
func (r *localRobot) ModuleAddress() (string, error) {
	return r.webSvc.ModuleAddress(), nil
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
	var err error
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
			r.packageManager, err = packages.NewCloudManager(cfg.Cloud, pb.NewPackageServiceClient(cloudConn), cfg.PackagePath, logger)
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
	r.frameSvc, err = framesystem.New(ctx, resource.Dependencies{}, logger)
	if err != nil {
		return nil, err
	}
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

	// Once web service is started, start module manager
	r.manager.startModuleManager(r.webSvc.ModuleAddress(), r.removeOrphanedResources, cfg.UntrustedEnv, logger)

	r.activeBackgroundWorkers.Add(1)
	r.configTicker = time.NewTicker(5 * time.Second)
	// This goroutine tries to complete the config and update weak dependencies
	// if any resources are not configured. It executes every 5 seconds or when
	// manually triggered. Manual triggers are sent when changes in remotes are
	// detected and in testing.
	goutils.ManagedGo(func() {
		for {
			if closeCtx.Err() != nil {
				return
			}

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

	r.mostRecentCfg = config.Config{}
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

// removeOrphanedResources is called by the module manager to remove resources
// orphaned due to module crashes.
func (r *localRobot) removeOrphanedResources(ctx context.Context,
	rNames []resource.Name,
) {
	r.manager.markResourcesRemoved(rNames, nil)
	if err := r.manager.removeMarkedAndClose(ctx, nil); err != nil {
		r.logger.Errorw("error removing and closing marked resources",
			"error", err)
	}
	r.updateWeakDependents(ctx)
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
			if r.lastWeakDependentsRound.Load() <= node.UpdatedAt() {
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
	visionServices := map[resource.Name]resource.Resource{}
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
		case n.API.SubtypeName == vision.API.SubtypeName:
			visionServices[n] = res
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
		case internal.VisionDependencyWildcardMatcher:
			match(visionServices)
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
		return nil, errors.Errorf("unknown resource type: API %q with model %q not registered", resName.API, conf.Model)
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
	r.lastWeakDependentsRound.Store(r.manager.resources.LastUpdatedTime())

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

	timeout := utils.GetResourceConfigurationTimeout(r.logger)
	// NOTE(erd): this is intentionally hard coded since these services are treated specially with
	// how they request dependencies or consume the robot's config. We should make an effort to
	// formalize these as servcices that while internal, obey the reconfigure lifecycle.
	// For example, the framesystem should depend on all input enabled components while the web
	// service depends on all resources.
	// For now, we pass all resources and empty configs.
	processInternalResources := func(resName resource.Name, res resource.Resource, resChan chan struct{}) {
		ctxWithTimeout, timeoutCancel := context.WithTimeout(ctx, timeout)
		defer timeoutCancel()
		r.reconfigureWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer func() {
				resChan <- struct{}{}
				r.reconfigureWorkers.Done()
			}()
			switch resName {
			case web.InternalServiceName:
				if err := res.Reconfigure(ctxWithTimeout, allResources, resource.Config{}); err != nil {
					r.Logger().Errorw("failed to reconfigure internal service", "service", resName, "error", err)
				}
			case framesystem.InternalServiceName:
				fsCfg, err := r.FrameSystemConfig(ctxWithTimeout)
				if err != nil {
					r.Logger().Errorw("failed to reconfigure internal service", "service", resName, "error", err)
					break
				}
				if err := res.Reconfigure(ctxWithTimeout, components, resource.Config{ConvertedAttributes: fsCfg}); err != nil {
					r.Logger().Errorw("failed to reconfigure internal service", "service", resName, "error", err)
				}
			case packages.InternalServiceName, cloud.InternalServiceName:
			default:
				r.logger.Warnw("do not know how to reconfigure internal service", "service", resName)
			}
		})

		select {
		case <-resChan:
		case <-ctxWithTimeout.Done():
			if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
				r.logger.Warn(resource.NewBuildTimeoutError(resName))
			}
		case <-ctx.Done():
			return
		}
	}

	for resName, res := range internalResources {
		select {
		case <-ctx.Done():
			return
		default:
		}
		resChan := make(chan struct{}, 1)
		resName := resName
		res := res
		processInternalResources(resName, res, resChan)
	}

	updateResourceWeakDependents := func(ctx context.Context, conf resource.Config) {
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
		r.Logger().Debugw("handling weak update for resource", "resource", resName)
		deps, err := r.getDependencies(ctx, resName, resNode)
		if err != nil {
			r.Logger().Errorw("failed to get dependencies during weak update; skipping", "resource", resName, "error", err)
			return
		}
		if err := res.Reconfigure(ctx, deps, conf); err != nil {
			r.Logger().Errorw("failed to reconfigure resource with weak dependencies", "resource", resName, "error", err)
		}
	}

	cfg := r.Config()
	for _, conf := range append(cfg.Components, cfg.Services...) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conf := conf
		ctxWithTimeout, timeoutCancel := context.WithTimeout(ctx, timeout)
		defer timeoutCancel()
		resChan := make(chan struct{}, 1)
		r.reconfigureWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer func() {
				resChan <- struct{}{}
				r.reconfigureWorkers.Done()
			}()
			updateResourceWeakDependents(ctxWithTimeout, conf)
		})
		select {
		case <-resChan:
		case <-ctxWithTimeout.Done():
			if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
				r.logger.Warn(resource.NewBuildTimeoutError(conf.ResourceName()))
			}
		case <-ctx.Done():
			return
		}
	}
}

// Config returns the info of each individual part that makes up the frame system
// The output of this function is to be sent over GRPC to the client, so the client
// can build its frame system. requests the remote components from the remote's frame system service.
func (r *localRobot) FrameSystemConfig(ctx context.Context) (*framesystem.Config, error) {
	localParts, err := r.getLocalFrameSystemParts()
	if err != nil {
		return nil, err
	}
	remoteParts, err := r.getRemoteFrameSystemParts(ctx)
	if err != nil {
		return nil, err
	}

	return &framesystem.Config{Parts: append(localParts, remoteParts...)}, nil
}

// getLocalFrameSystemParts collects and returns the physical parts of the robot that may have frame info,
// excluding remote robots and services, etc from the robot's config.Config.
func (r *localRobot) getLocalFrameSystemParts() ([]*referenceframe.FrameSystemPart, error) {
	cfg := r.Config()

	parts := make([]*referenceframe.FrameSystemPart, 0)
	for _, component := range cfg.Components {
		if component.Frame == nil { // no Frame means dont include in frame system.
			continue
		}

		if component.Name == referenceframe.World {
			return nil, errors.Errorf("cannot give frame system part the name %s", referenceframe.World)
		}
		if component.Frame.Parent == "" {
			return nil, errors.Errorf("parent field in frame config for part %q is empty", component.Name)
		}
		cfgCopy := &referenceframe.LinkConfig{
			ID:          component.Frame.ID,
			Translation: component.Frame.Translation,
			Orientation: component.Frame.Orientation,
			Geometry:    component.Frame.Geometry,
			Parent:      component.Frame.Parent,
		}
		if cfgCopy.ID == "" {
			cfgCopy.ID = component.Name
		}
		model, err := r.extractModelFrameJSON(component.ResourceName())
		if err != nil && !errors.Is(err, referenceframe.ErrNoModelInformation) {
			// When we have non-nil errors here, it is because the resource is not yet available.
			// In this case, we will exclude it from the FS.
			// When it becomes available, it will be included.
			continue
		}
		lif, err := cfgCopy.ParseConfig()
		if err != nil {
			return nil, err
		}

		parts = append(parts, &referenceframe.FrameSystemPart{FrameConfig: lif, ModelFrame: model})
	}
	return parts, nil
}

func (r *localRobot) getRemoteFrameSystemParts(ctx context.Context) ([]*referenceframe.FrameSystemPart, error) {
	cfg := r.Config()

	remoteParts := make([]*referenceframe.FrameSystemPart, 0)
	for _, remoteCfg := range cfg.Remotes {
		// build the frame system part that connects remote world to base world
		if remoteCfg.Frame == nil { // skip over remote if it has no frame info
			r.logger.Debugf("remote %q has no frame config info, skipping", remoteCfg.Name)
			continue
		}
		lif, err := remoteCfg.Frame.ParseConfig()
		if err != nil {
			return nil, err
		}
		parentName := remoteCfg.Name + "_" + referenceframe.World
		lif.SetName(parentName)
		remoteParts = append(remoteParts, &referenceframe.FrameSystemPart{FrameConfig: lif})

		// get the parts from the remote itself
		remote, ok := r.RemoteByName(remoteCfg.Name)
		if !ok {
			return nil, errors.Errorf("cannot find remote robot %q", remoteCfg.Name)
		}
		remoteFsCfg, err := remote.FrameSystemConfig(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error from remote %q", remoteCfg.Name)
		}
		framesystem.PrefixRemoteParts(remoteFsCfg.Parts, remoteCfg.Name, parentName)
		remoteParts = append(remoteParts, remoteFsCfg.Parts...)
	}
	return remoteParts, nil
}

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func (r *localRobot) extractModelFrameJSON(name resource.Name) (referenceframe.Model, error) {
	part, err := r.ResourceByName(name)
	if err != nil {
		return nil, err
	}
	if framer, ok := part.(referenceframe.ModelFramer); ok {
		return framer.ModelFrame(), nil
	}
	return nil, referenceframe.ErrNoModelInformation
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
// possibly leak resources. The given config may be modified by Reconfigure.
func (r *localRobot) Reconfigure(ctx context.Context, newConfig *config.Config) {
	var allErrs error

	// Sync Packages before reconfiguring rest of robot and resolving references to any packages
	// in the config.
	// TODO(RSDK-1849): Make this non-blocking so other resources that do not require packages can run before package sync finishes.
	// TODO(RSDK-2710) this should really use Reconfigure for the package and should allow itself to check
	// if anything has changed.
	err := r.packageManager.Sync(ctx, newConfig.Packages)
	if err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}

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

	// Now that we have the new config and all references are resolved, diff it
	// with the current generated config to see what has changed
	diff, err := config.DiffConfigs(*r.Config(), *newConfig, r.revealSensitiveConfigDiffs)
	if err != nil {
		r.logger.Errorw("error diffing the configs", "error", err)
		return
	}
	if diff.ResourcesEqual {
		return
	}
	// Set mostRecentConfig if resources were not equal.
	r.mostRecentCfg = *newConfig

	// We need to pre-add the new modules so that resource validation can check against the new models
	// TODO(RSDK-4383) These lines are taken from uppdateResources() and should be refactored as part of this bugfix
	for _, mod := range diff.Added.Modules {
		if err := mod.Validate(""); err != nil {
			r.manager.logger.Errorw("module config validation error; skipping", "module", mod.Name, "error", err)
			continue
		}
		if err := r.manager.moduleManager.Add(ctx, mod); err != nil {
			r.manager.logger.Errorw("error adding module", "module", mod.Name, "error", err)
			continue
		}
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

	// First we mark diff.Removed resources and their children for removal.
	processesToClose, resourcesToCloseBeforeComplete, _ := r.manager.markRemoved(ctx, diff.Removed, r.logger)

	// Second we update the resource graph and stop any removed processes.
	allErrs = multierr.Combine(allErrs, r.manager.updateResources(ctx, diff))
	allErrs = multierr.Combine(allErrs, processesToClose.Stop())

	// Third we attempt to Close resources.
	alreadyClosed := make(map[resource.Name]struct{}, len(resourcesToCloseBeforeComplete))
	for _, res := range resourcesToCloseBeforeComplete {
		allErrs = multierr.Combine(allErrs, r.manager.closeResource(ctx, res))
		// avoid a double close later
		alreadyClosed[res.Name()] = struct{}{}
	}

	// Fourth we attempt to complete the config (see function for details) and
	// update weak dependents.
	r.manager.completeConfig(ctx, r)
	r.updateWeakDependents(ctx)

	// Finally we actually remove marked resources and Close any that are
	// still unclosed.
	if err := r.manager.removeMarkedAndClose(ctx, alreadyClosed); err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}

	// cleanup unused packages after all old resources have been closed above. This ensures
	// processes are shutdown before any files are deleted they are using.
	allErrs = multierr.Combine(allErrs, r.packageManager.Cleanup(ctx))

	if allErrs != nil {
		r.logger.Errorw("the following errors were gathered during reconfiguration", "errors", allErrs)
	}
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
