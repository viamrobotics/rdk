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
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/utils"
)

type internalServiceName string

const (
	webName         internalServiceName = "web"
	framesystemName internalServiceName = "framesystem"
)

var _ = robot.LocalRobot(&localRobot{})

// localRobot satisfies robot.LocalRobot and defers most
// logic to its manager.
type localRobot struct {
	mu         sync.Mutex
	manager    *resourceManager
	config     *config.Config
	operations *operation.Manager
	logger     golog.Logger

	// services internal to a localRobot. Currently just web, more to come.
	internalServices map[internalServiceName]interface{}

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()

	remotesChanged chan string
	closeContext   context.Context
	triggerConfig  chan bool
	configTimer    *time.Ticker
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

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *localRobot) Close(ctx context.Context) error {
	for _, svc := range r.internalServices {
		if err := goutils.TryClose(ctx, svc); err != nil {
			return err
		}
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
	return r.manager.Close(ctx)
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

		sr, ok := res.(resource.Stoppable)
		if ok {
			err = sr.Stop(ctx, extra[name])
			if err != nil {
				resourceErrs = append(resourceErrs, name.Name)
			}
		}

		// TODO[njooma]: OldStoppable - Will be deprecated
		osr, ok := res.(resource.OldStoppable)
		if ok {
			err = osr.Stop(ctx)
			if err != nil {
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

// remoteNameByResource returns the remote the resource is pulled from, if found.
// False can mean either the resource doesn't exist or is local to the robot.
func remoteNameByResource(resourceName resource.Name) (string, bool) {
	if !resourceName.ContainsRemoteNames() {
		return "", false
	}
	remote := strings.Split(string(resourceName.Remote), ":")
	return remote[0], true
}

func (r *localRobot) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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

		s, err := remote.GetStatus(ctx, remoteRNames)
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
				tlsConfig:          cfg.Network.TLSConfig,
			},
			logger,
		),
		operations:              operation.NewManager(),
		logger:                  logger,
		remotesChanged:          make(chan string),
		activeBackgroundWorkers: &sync.WaitGroup{},
		closeContext:            closeCtx,
		cancelBackgroundWorkers: cancel,
		triggerConfig:           make(chan bool),
		configTimer:             nil,
	}

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(context.Background()); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()
	// start process manager early
	if err := r.manager.processManager.Start(ctx); err != nil {
		return nil, err
	}
	// default services
	for _, name := range resource.DefaultServices {
		cfg := config.Service{
			Namespace: name.Namespace,
			Type:      config.ServiceType(name.ResourceSubtype),
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
		}
	}, r.activeBackgroundWorkers.Done)

	r.internalServices = make(map[internalServiceName]interface{})
	r.internalServices[webName] = web.New(ctx, r, logger, rOpts.webOptions...)
	r.internalServices[framesystemName] = framesystem.New(ctx, r, logger)

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
func New(ctx context.Context, cfg *config.Config, logger golog.Logger, opts ...Option) (robot.LocalRobot, error) {
	return newWithResources(ctx, cfg, nil, logger, opts...)
}

func (r *localRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ServiceLookup(rName.Subtype)
	if f == nil {
		return nil, errors.Errorf("unknown service type: %s", rName.Subtype)
	}
	svc, err := f.Constructor(ctx, r, config, r.logger)
	if err != nil {
		return nil, err
	}
	c := registry.ResourceSubtypeLookup(rName.Subtype)
	if c == nil || c.Reconfigurable == nil {
		return svc, nil
	}
	return c.Reconfigurable(svc)
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

func (r *localRobot) newResource(ctx context.Context, config config.Component) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ComponentLookup(rName.Subtype, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown component subtype: %s and/or model: %s", rName.Subtype, config.Model)
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
	return c.Reconfigurable(newResource)
}

func (r *localRobot) updateDefaultServices(ctx context.Context) {
	resources := map[resource.Name]interface{}{}
	for _, n := range r.ResourceNames() {
		// TODO(RSDK-333) if not found, could mean a name clash or a remote service
		res, err := r.ResourceByName(n)
		if err != nil {
			r.Logger().Debugw("not found while grabbing all resources during default svc refresh", "resource", res, "error", err)
		}
		resources[n] = res
	}

	for _, name := range resource.DefaultServices {
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
func (r *localRobot) FrameSystemConfig(ctx context.Context, additionalTransforms []*commonpb.Transform) (framesystemparts.Parts, error) {
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
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	framesystem, err := r.fsService()
	if err != nil {
		return nil, err
	}

	return framesystem.TransformPose(ctx, pose, dst, additionalTransforms)
}

// RobotFromConfigPath is a helper to read and process a config given its path and then create a robot based on it.
func RobotFromConfigPath(ctx context.Context, cfgPath string, logger golog.Logger, opts ...Option) (robot.LocalRobot, error) {
	cfg, err := config.Read(ctx, cfgPath, logger)
	if err != nil {
		logger.Fatal("cannot read config")
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
	return newWithResources(ctx, &config.Config{}, resources, logger)
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
			r.logger.Warnw("no discovery function registered", "subtype", q.SubtypeName, "model", q.Model)
			continue
		}

		if discoveryFunction != nil {
			discovered, err := discoveryFunction(ctx)
			if err != nil {
				return nil, &discovery.DiscoverError{q}
			}
			discoveries = append(discoveries, discovery.Discovery{Query: q, Results: discovered})
		}
	}
	return discoveries, nil
}

func dialRobotClient(ctx context.Context,
	config config.Remote,
	logger golog.Logger,
	dialOpts ...rpc.DialOption,
) (*client.RobotClient, error) {
	connectionCheckInterval := config.ConnectionCheckInterval
	if connectionCheckInterval == 0 {
		connectionCheckInterval = 10 * time.Second
	}
	reconnectInterval := config.ReconnectInterval
	if reconnectInterval == 0 {
		reconnectInterval = 1 * time.Second
	}

	robotClient, err := client.New(
		ctx,
		config.Address,
		logger,
		client.WithDialOptions(dialOpts...),
		client.WithCheckConnectedEvery(connectionCheckInterval),
		client.WithReconnectEvery(reconnectInterval),
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
	var allErrs error
	diff, err := config.DiffConfigs(r.config, newConfig)
	if err != nil {
		r.logger.Errorw("error diffing the configs", "error", err)
		return
	}
	if diff.ResourcesEqual {
		return
	}
	r.logger.Debugf("(re)configuring with %+v", diff)
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
		// we are trying to locate a resource that is set as a dependency but do not exist yet
		r.logger.Debugw("processing unknown  resource", "name", name)
		return resource.NameFromSubtype(unknownSubtype, name), true
	})
	if err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}
	r.config = newConfig
	allErrs = multierr.Combine(allErrs, filtered.Close(ctx))
	// Third we attempt to complete the config (see function for details)
	r.manager.completeConfig(ctx, r)
	r.updateDefaultServices(ctx)
	if allErrs != nil {
		r.logger.Errorw("the following errors were gathered during reconfiguration", "errors", allErrs)
	}
}
