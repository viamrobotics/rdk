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
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/module/modmanager"
	modif "go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/utils"
)

type internalServiceName string

const (
	webName            internalServiceName = "web"
	framesystemName    internalServiceName = "framesystem"
	packageManagerName internalServiceName = "packagemanager"
)

var _ = robot.LocalRobot(&localRobot{})

// localRobot satisfies robot.LocalRobot and defers most
// logic to its manager.
type localRobot struct {
	mu             sync.Mutex
	manager        *resourceManager
	config         *config.Config
	operations     *operation.Manager
	modules        modif.ModuleManager
	sessionManager session.Manager
	packageManager packages.ManagerSyncer
	cloudConn      rpc.ClientConn
	logger         golog.Logger

	// services internal to a localRobot. Currently just web, more to come.
	internalServices     map[internalServiceName]interface{}
	defaultServicesNames map[resource.Subtype]resource.Name

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()

	remotesChanged             chan string
	closeContext               context.Context
	triggerConfig              chan bool
	configTimer                *time.Ticker
	revealSensitiveConfigDiffs bool
}

// webService returns the localRobot's web service. Raises if the service has not been initialized.
func (r *localRobot) webService() (web.Service, error) {
	svc := r.internalServices[webName]
	if svc == nil {
		return nil, errors.New("web service not initialized")
	}

	webSvc, ok := svc.(web.Service)
	if !ok {
		return nil, errors.New("unexpected service associated with web InternalServiceName")
	}
	return webSvc, nil
}

// fsService returns the localRobot's web service. Raises if the service has not been initialized.
func (r *localRobot) fsService() (framesystem.Service, error) {
	svc := r.internalServices[framesystemName]
	if svc == nil {
		return nil, errors.New("framesystem service not initialized")
	}

	framesystemSvc, ok := svc.(framesystem.Service)
	if !ok {
		return nil, errors.New("unexpected service associated with framesystem internalServiceName")
	}
	return framesystemSvc, nil
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *localRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.manager.RemoteByName(name)
}

// ResourceByName returns a resource by name. If it does not exist
// nil is returned.
func (r *localRobot) ResourceByName(name resource.Name) (interface{}, error) {
	return r.manager.ResourceByName(name)
}

// RemoteNames returns the name of all known remote robots.
func (r *localRobot) RemoteNames() []string {
	return r.manager.RemoteNames()
}

// ResourceNames returns the name of all known resources.
func (r *localRobot) ResourceNames() []resource.Name {
	return r.manager.ResourceNames()
}

// ResourceRPCSubtypes returns all known resource RPC subtypes in use.
func (r *localRobot) ResourceRPCSubtypes() []resource.RPCSubtype {
	return r.manager.ResourceRPCSubtypes()
}

// ProcessManager returns the process manager for the robot.
func (r *localRobot) ProcessManager() pexec.ProcessManager {
	return r.manager.processManager
}

// OperationManager returns the operation manager for the robot.
func (r *localRobot) OperationManager() *operation.Manager {
	return r.operations
}

// ModuleManager returns the module manager for the robot.
func (r *localRobot) ModuleManager() modif.ModuleManager {
	return r.modules
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
	web, err := r.webService()
	if err == nil {
		web.Stop()
	}
	for s, svc := range r.internalServices {
		if s == webName {
			continue
		}
		err = multierr.Combine(err, goutils.TryClose(ctx, svc))
	}
	if r.cancelBackgroundWorkers != nil {
		close(r.remotesChanged)
		r.cancelBackgroundWorkers()
		r.cancelBackgroundWorkers = nil
		if r.configTimer != nil {
			r.configTimer.Stop()
		}
		close(r.triggerConfig)
	}
	r.activeBackgroundWorkers.Wait()

	if r.cloudConn != nil {
		err = multierr.Combine(err, r.cloudConn.Close())
	}
	if r.manager != nil {
		err = multierr.Combine(err, r.manager.Close(ctx, r))
	}
	if r.modules != nil {
		err = multierr.Combine(err, r.modules.Close(ctx))
	}
	if r.packageManager != nil {
		err = multierr.Combine(err, r.packageManager.Close())
	}

	err = multierr.Combine(
		err,
		goutils.TryClose(ctx, web),
	)
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

		if err := resource.StopResource(ctx, res, extra[name]); err != nil {
			resourceErrs = append(resourceErrs, name.Name)
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
	cfgCpy.Components = append([]config.Component{}, cfgCpy.Components...)

	return &cfgCpy, nil
}

// Logger returns the logger the robot is using.
func (r *localRobot) Logger() golog.Logger {
	return r.logger
}

// StartWeb starts the web server, will return an error if server is already up.
func (r *localRobot) StartWeb(ctx context.Context, o weboptions.Options) (err error) {
	webSvc, err := r.webService()
	if err != nil {
		return err
	}
	return webSvc.Start(ctx, o)
}

// StopWeb stops the web server, will be a noop if server is not up.
func (r *localRobot) StopWeb() error {
	webSvc, err := r.webService()
	if err != nil {
		return err
	}
	return webSvc.Close()
}

// WebAddress return the web service's address.
func (r *localRobot) WebAddress() (string, error) {
	webSvc, err := r.webService()
	if err != nil {
		return "", err
	}
	return webSvc.Address(), nil
}

// ModuleAddress return the module service's address.
func (r *localRobot) ModuleAddress() (string, error) {
	webSvc, err := r.webService()
	if err != nil {
		return "", err
	}
	return webSvc.ModuleAddress(), nil
}

// remoteNameByResource returns the remote the resource is pulled from, if found.
// False can mean either the resource doesn't exist or is local to the robot.
func remoteNameByResource(resourceName resource.Name) (string, bool) {
	if !resourceName.ContainsRemoteNames() {
		return "", false
	}
	remote := strings.Split(string(resourceName.Remote), ":")
	return remote[0], true
}

func (r *localRobot) Status(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	r.mu.Lock()
	resources := make(map[resource.Name]interface{}, len(r.manager.resources.Names()))
	for _, name := range r.ResourceNames() {
		resource, err := r.ResourceByName(name)
		if err != nil {
			return nil, utils.NewResourceNotFoundError(name)
		}
		resources[name] = resource
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
			resource, ok := resources[name]
			if !ok {
				return nil, utils.NewResourceNotFoundError(name)
			}
			// if resource subtype has an associated CreateStatus method, use that
			// otherwise return an empty status
			var status interface{} = map[string]interface{}{}
			var err error
			subtype := registry.ResourceSubtypeLookup(name.Subtype)
			if subtype != nil && subtype.Status != nil {
				status, err = subtype.Status(ctx, resource)
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

func (r *localRobot) updateDefaultServiceNames(cfg *config.Config) *config.Config {
	// See if default service already exists in the config
	seen := make(map[resource.Subtype]bool)
	for _, name := range resource.DefaultServices {
		seen[name.Subtype] = false
		r.defaultServicesNames[name.Subtype] = name
	}
	// Mark default service subtypes in the map as true
	for _, val := range cfg.Services {
		if _, ok := seen[val.ResourceName().Subtype]; ok {
			seen[val.ResourceName().Subtype] = true
			r.defaultServicesNames[val.ResourceName().Subtype] = val.ResourceName()
		}
	}
	// default services added if they are not already defined in the config
	for _, name := range resource.DefaultServices {
		if seen[name.Subtype] {
			continue
		}
		svcCfg := config.Service{
			Name:      name.Name,
			Model:     resource.DefaultServiceModel,
			Namespace: name.Namespace,
			Type:      name.ResourceSubtype,
		}
		cfg.Services = append(cfg.Services, svcCfg)
	}
	return cfg
}

func newWithResources(
	ctx context.Context,
	cfg *config.Config,
	resources map[resource.Name]interface{},
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
		remotesChanged:             make(chan string),
		activeBackgroundWorkers:    &sync.WaitGroup{},
		closeContext:               closeCtx,
		cancelBackgroundWorkers:    cancel,
		defaultServicesNames:       make(map[resource.Subtype]resource.Name),
		triggerConfig:              make(chan bool),
		configTimer:                nil,
		revealSensitiveConfigDiffs: rOpts.revealSensitiveConfigDiffs,
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
		var err error
		timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		r.cloudConn, err = config.CreateNewGRPCClient(timeOutCtx, cfg.Cloud, logger)
		cancel()
		if err == nil {
			r.packageManager, err = packages.NewCloudManager(pb.NewPackageServiceClient(r.cloudConn), cfg.PackagePath, logger)
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

	r.internalServices = make(map[internalServiceName]interface{})
	r.internalServices[webName] = web.New(r, logger, rOpts.webOptions...)
	r.internalServices[framesystemName] = framesystem.New(ctx, r, logger)
	r.internalServices[packageManagerName] = r.packageManager

	webSvc, err := r.webService()
	if err != nil {
		return nil, err
	}
	if err := webSvc.StartModule(ctx); err != nil {
		return nil, err
	}

	modMgr, err := modmanager.NewManager(r)
	if err != nil {
		return nil, err
	}
	r.modules = modMgr

	for _, mod := range cfg.Modules {
		err := r.modules.Add(ctx, mod)
		if err != nil {
			return nil, err
		}
	}

	// See if default service already exists in the config
	seen := make(map[resource.Subtype]bool)
	for _, name := range resource.DefaultServices {
		seen[name.Subtype] = false
		r.defaultServicesNames[name.Subtype] = name
	}
	// Mark default service subtypes in the map as true
	for _, val := range cfg.Services {
		if _, ok := seen[val.ResourceName().Subtype]; ok {
			seen[val.ResourceName().Subtype] = true
			r.defaultServicesNames[val.ResourceName().Subtype] = val.ResourceName()
		}
	}
	// default services added if they are not already defined in the config
	for _, name := range resource.DefaultServices {
		if seen[name.Subtype] {
			continue
		}
		cfg := config.Service{
			Name:      name.Name,
			Model:     resource.DefaultServiceModel,
			Namespace: name.Namespace,
			Type:      name.ResourceSubtype,
		}
		svc, err := r.newService(ctx, cfg)
		if err != nil {
			logger.Errorw("failed to add default service", "error", err, "service", name)
			continue
		}
		r.manager.addResource(name, svc)
	}

	r.activeBackgroundWorkers.Add(1)
	// this goroutine listen for changes in connection status of a remote
	goutils.ManagedGo(func() {
		for {
			if closeCtx.Err() != nil {
				return
			}
			select {
			case <-closeCtx.Done():
				return
			case n, ok := <-r.remotesChanged:
				if !ok {
					return
				}
				if rr, ok := r.manager.RemoteByName(n); ok {
					rn := fromRemoteNameToRemoteNodeName(n)
					r.manager.updateRemoteResourceNames(ctx, rn, rr, r)
					r.updateDefaultServices(ctx)
				}
			}
		}
	}, r.activeBackgroundWorkers.Done)

	r.activeBackgroundWorkers.Add(1)
	r.configTimer = time.NewTicker(25 * time.Second)
	// this goroutine tries to complete the config if any resources are still unconfigured, it execute on a timer or via a channel
	goutils.ManagedGo(func() {
		for {
			if closeCtx.Err() != nil {
				return
			}
			select {
			case <-closeCtx.Done():
				return
			case <-r.triggerConfig:
			case <-r.configTimer.C:
			}
			if r.manager.anyResourcesNotConfigured() {
				r.manager.completeConfig(closeCtx, r)
				r.updateDefaultServices(ctx)
			}
			if r.manager.updateRemotesResourceNames(ctx, r) {
				r.updateDefaultServices(ctx)
			}
		}
	}, r.activeBackgroundWorkers.Done)

	r.config = &config.Config{}

	r.Reconfigure(ctx, cfg)

	for name, res := range resources {
		r.manager.addResource(name, res)
	}

	if len(resources) != 0 {
		r.updateDefaultServices(ctx)
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

func (r *localRobot) newService(ctx context.Context, config config.Service) (res interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(errors.Errorf("%v", r), "panic creating service")
		}
	}()
	rName := config.ResourceName()
	f := registry.ServiceLookup(rName.Subtype, config.Model)
	// If service model/type not found then print list of valid models they can choose from
	if f == nil {
		validModels := registry.FindValidServiceModels(rName)
		return nil, errors.Errorf("unknown service subtype: %s and/or model: %s use one of the following valid models: %s",
			rName.Subtype, config.Model, validModels)
	}

	deps, err := r.getDependencies(rName)
	if err != nil {
		return nil, err
	}
	c := registry.ResourceSubtypeLookup(rName.Subtype)

	// If MaxInstance equals zero then there is not a limit on the number of services
	if c.MaxInstance != 0 {
		if err := r.checkMaxInstance(rName.Subtype, c.MaxInstance); err != nil {
			return nil, err
		}
	}
	var svc interface{}
	if f.Constructor != nil {
		svc, err = f.Constructor(ctx, deps, config, r.logger)
		if err != nil {
			return nil, err
		}
	} else {
		svc, err = f.RobotConstructor(ctx, r, config, r.logger)
		if err != nil {
			return nil, err
		}
	}

	if c == nil || c.Reconfigurable == nil {
		return svc, nil
	}
	return c.Reconfigurable(svc, rName)
}

// getDependencies derives a collection of dependencies from a robot for a given
// component's name. We don't use the resource manager for this information since
// it is not be constructed at this point.
func (r *localRobot) getDependencies(rName resource.Name) (registry.Dependencies, error) {
	deps := make(registry.Dependencies)
	for _, dep := range r.manager.resources.GetAllParentsOf(rName) {
		r, err := r.ResourceByName(dep)
		if err != nil {
			return nil, &registry.DependencyNotReadyError{Name: dep.Name}
		}
		deps[dep] = r
	}

	return deps, nil
}

func (r *localRobot) newResource(ctx context.Context, config config.Component) (res interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(errors.Errorf("%v", r), "panic creating resource")
		}
	}()
	rName := config.ResourceName()
	f := registry.ComponentLookup(rName.Subtype, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown component type: %s and/or model: %s", rName.Subtype, config.Model)
	}

	deps, err := r.getDependencies(rName)
	if err != nil {
		return nil, err
	}

	var newResource interface{}
	if f.Constructor != nil {
		newResource, err = f.Constructor(ctx, deps, config, r.logger)
	} else {
		r.logger.Warnw("using legacy constructor", "subtype", rName.Subtype, "model", config.Model)
		newResource, err = f.RobotConstructor(ctx, r, config, r.logger)
	}

	if err != nil {
		return nil, err
	}

	c := registry.ResourceSubtypeLookup(rName.Subtype)
	if c == nil || c.Reconfigurable == nil {
		return newResource, nil
	}
	wrapped, err := c.Reconfigurable(newResource, rName)
	if err != nil {
		return nil, multierr.Combine(err, goutils.TryClose(ctx, newResource))
	}
	return wrapped, nil
}

func (r *localRobot) updateDefaultServices(ctx context.Context) {
	resources := map[resource.Name]interface{}{}
	for _, n := range r.ResourceNames() {
		res, err := r.ResourceByName(n)
		if err != nil {
			r.Logger().Debugw("not found while grabbing all resources during default svc refresh", "resource", res, "error", err)
		}
		resources[n] = res
	}

	for _, name := range r.defaultServicesNames {
		svc, err := r.ResourceByName(name)
		if err != nil {
			r.Logger().Errorw("resource not found", "error", utils.NewResourceNotFoundError(name))
			continue
		}
		if updateable, ok := svc.(resource.Updateable); ok {
			if err := updateable.Update(ctx, resources); err != nil {
				r.Logger().Errorw("failed to update resource", "resource", name, "error", err)
				continue
			}
		}
		if configUpdateable, ok := svc.(config.Updateable); ok {
			if err := configUpdateable.Update(ctx, r.config); err != nil {
				r.Logger().Errorw("config for service failed to update", "resource", name, "error", err)
				continue
			}
		}
	}

	for _, svc := range r.internalServices {
		if updateable, ok := svc.(resource.Updateable); ok {
			if err := updateable.Update(ctx, resources); err != nil {
				r.Logger().Errorw("failed to update internal service", "resource", svc, "error", err)
				continue
			}
		}
	}
}

// Refresh does nothing for now.
func (r *localRobot) Refresh(ctx context.Context) error {
	return nil
}

// FrameSystemConfig returns the info of each individual part that makes up a robot's frame system.
func (r *localRobot) FrameSystemConfig(
	ctx context.Context,
	additionalTransforms []*referenceframe.LinkInFrame,
) (framesystemparts.Parts, error) {
	framesystem, err := r.fsService()
	if err != nil {
		return nil, err
	}

	return framesystem.Config(ctx, additionalTransforms)
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (r *localRobot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	framesystem, err := r.fsService()
	if err != nil {
		return nil, err
	}

	return framesystem.TransformPose(ctx, pose, dst, additionalTransforms)
}

// TransformPointCloud will transform the pointcloud to the desired frame in the robot's frame system.
// Do not move the robot between the generation of the initial pointcloud and the receipt
// of the transformed pointcloud because that will make the transformations inaccurate.
func (r *localRobot) TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string,
) (pointcloud.PointCloud, error) {
	framesystem, err := r.fsService()
	if err != nil {
		return nil, err
	}

	return framesystem.TransformPointCloud(ctx, srcpc, srcName, dstName)
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
	resources map[resource.Name]interface{},
	logger golog.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	return newWithResources(ctx, &config.Config{}, resources, logger, opts...)
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (r *localRobot) DiscoverComponents(ctx context.Context, qs []discovery.Query) ([]discovery.Discovery, error) {
	// dedupe queries
	deduped := make(map[discovery.Query]struct{}, len(qs))
	for _, q := range qs {
		deduped[q] = struct{}{}
	}

	discoveries := make([]discovery.Discovery, 0, len(deduped))
	for q := range deduped {
		discoveryFunction, ok := registry.DiscoveryFunctionLookup(q)
		if !ok {
			r.logger.Warnw("no discovery function registered", "subtype", q.API, "model", q.Model)
			continue
		}

		if discoveryFunction != nil {
			discovered, err := discoveryFunction(ctx)
			if err != nil {
				return nil, &discovery.DiscoverError{Query: q}
			}
			discoveries = append(discoveries, discovery.Discovery{Query: q, Results: discovered})
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
func (r *localRobot) Reconfigure(ctx context.Context, newConfig *config.Config) {
	// Before reconfiguring, go through resources in newConfig, call Validate on all
	// modularized resources, and store those resources' implicit dependencies.
	for i, c := range newConfig.Components {
		if r.modules.Provides(c) {
			implicitDeps, err := r.modules.ValidateConfig(ctx, c)
			if err != nil {
				r.logger.Errorw("Modular config validation error found in component: "+c.Name, "error", err)
				continue
			}

			// Modify component to add its implicit dependencies.
			newConfig.Components[i].ImplicitDependsOn = implicitDeps
		}
	}
	for i, s := range newConfig.Services {
		c := config.ServiceConfigToShared(s)
		if r.modules.Provides(c) {
			implicitDeps, err := r.modules.ValidateConfig(ctx, c)
			if err != nil {
				r.logger.Errorw("Modular config validation error found in service: "+s.Name, "error", err)
				continue
			}

			// Modify service to add its implicit dependencies.
			newConfig.Services[i].ImplicitDependsOn = implicitDeps
		}
	}

	var allErrs error

	newConfig = r.updateDefaultServiceNames(newConfig)

	// Sync Packages before reconfiguring rest of robot and resolving references to any packages
	// in the config.
	// TODO(RSDK-1849): Make this non-blocking so other resources that do not require packages can run before package sync finishes.
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

	if r.revealSensitiveConfigDiffs {
		r.logger.Debugf("(re)configuring with %+v", diff)
	}

	// First we remove resources and their children that are not in the graph.
	filtered, err := r.manager.FilterFromConfig(ctx, diff.Removed, r.logger)
	if err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}
	// Second we update the resource graph.
	// We pass a search function to look for dependencies, we should find them either in the current config or in the modified.
	err = r.manager.updateResources(ctx, diff, func(name string) (resource.Name, bool) {
		// first look in the current config if anything can be found
		for _, c := range r.config.Components {
			if c.Name == name {
				return c.ResourceName(), true
			}
		}
		// then look into what was added
		for _, c := range diff.Added.Components {
			if c.Name == name {
				return c.ResourceName(), true
			}
		}
		for _, s := range r.config.Services {
			if s.Name == name {
				return s.ResourceName(), true
			}
		}
		// then look into what was added
		for _, s := range diff.Added.Services {
			if s.Name == name {
				return s.ResourceName(), true
			}
		}

		// we are trying to locate a resource that is set as a dependency but do not exist yet
		r.logger.Debugw("processing unknown  resource", "name", name)
		return resource.NameFromSubtype(unknownSubtype, name), true
	})
	if err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}
	r.config = newConfig
	allErrs = multierr.Combine(allErrs, filtered.Close(ctx, r))
	// Third we attempt to complete the config (see function for details)
	r.manager.completeConfig(ctx, r)
	r.updateDefaultServices(ctx)

	// cleanup unused packages after all old resources have been closed above. This ensures
	// processes are shutdown before any files are deleted they are using.
	allErrs = multierr.Combine(allErrs, r.packageManager.Cleanup(ctx))

	if allErrs != nil {
		r.logger.Errorw("the following errors were gathered during reconfiguration", "errors", allErrs)
	}
}

func (r *localRobot) replacePackageReferencesWithPaths(cfg *config.Config) error {
	walkConvertedAttributes := func(convertedAttributes interface{}, allErrs error) (interface{}, error) {
		// Replace all package references with the actual path containing the package
		// on the robot.
		if walker, ok := convertedAttributes.(config.Walker); ok {
			newAttrs, err := walker.Walk(packages.NewPackagePathVisitor(r.packageManager))
			if err != nil {
				allErrs = multierr.Combine(allErrs, err)
				return convertedAttributes, allErrs
			}
			convertedAttributes = newAttrs
		}
		return convertedAttributes, allErrs
	}

	var allErrs error
	for i, s := range cfg.Services {
		s.ConvertedAttributes, allErrs = walkConvertedAttributes(s.ConvertedAttributes, allErrs)
		cfg.Services[i] = s
	}

	for i, c := range cfg.Components {
		c.ConvertedAttributes, allErrs = walkConvertedAttributes(c.ConvertedAttributes, allErrs)
		cfg.Components[i] = c
	}

	return allErrs
}

// checkMaxInstance checks to see if the local robot has reached the maximum number of a specific service type that are local.
func (r *localRobot) checkMaxInstance(subtype resource.Subtype, max int) error {
	maxInstance := 0
	for _, n := range r.ResourceNames() {
		if n.Subtype == subtype && !n.ContainsRemoteNames() {
			maxInstance++
			if maxInstance == max {
				return errors.Errorf("Max instance number reached for service type: %s", subtype)
			}
		}
	}
	return nil
}
