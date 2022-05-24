// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
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
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

type internalServiceName string

const (
	webName         internalServiceName = "web"
	framesystemName internalServiceName = "framesystem"
)

var (
	_ = robot.LocalRobot(&localRobot{})

	// defaultSvc is a list of default robot services.
	defaultSvc = []resource.Name{
		sensors.Name,
		status.Name,
		datamanager.Name,
		vision.Name,
	}
)

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

	return r.manager.Close(ctx)
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

func newWithResources(
	ctx context.Context,
	cfg *config.Config,
	resources map[resource.Name]interface{},
	logger golog.Logger,
) (robot.LocalRobot, error) {
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
		operations: operation.NewManager(),
		logger:     logger,
	}

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(context.Background()); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()
	r.config = cfg

	// default services
	for _, name := range defaultSvc {
		cfg := config.Service{Type: config.ServiceType(name.ResourceSubtype)}
		svc, err := r.newService(ctx, cfg)
		if err != nil {
			return nil, err
		}
		r.manager.addResource(name, svc)
	}

	r.internalServices = make(map[internalServiceName]interface{})
	r.internalServices[webName] = web.New(ctx, r, logger)
	r.internalServices[framesystemName] = framesystem.New(ctx, r, logger)
	if err := r.manager.processConfig(ctx, cfg, r, logger); err != nil {
		return nil, err
	}

	for name, res := range resources {
		r.manager.addResource(name, res)
	}

	// update default services - done here so that all resources have been created and can be addressed.
	if err := r.updateDefaultServices(ctx); err != nil {
		return nil, err
	}

	successful = true
	return r, nil
}

// New returns a new robot with parts sourced from the given config.
func New(ctx context.Context, cfg *config.Config, logger golog.Logger) (robot.LocalRobot, error) {
	return newWithResources(ctx, cfg, nil, logger)
}

func (r *localRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ServiceLookup(rName.Subtype)
	if f == nil {
		return nil, errors.Errorf("unknown service type: %s", rName.Subtype)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newResource(ctx context.Context, config config.Component) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ComponentLookup(rName.Subtype, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown component subtype: %s and/or model: %s", rName.Subtype, config.Model)
	}
	newResource, err := f.Constructor(ctx, r, config, r.logger)
	if err != nil {
		return nil, err
	}
	c := registry.ResourceSubtypeLookup(rName.Subtype)
	if c == nil || c.Reconfigurable == nil {
		return newResource, nil
	}
	return c.Reconfigurable(newResource)
}

// ConfigUpdateable is implemented when component/service of a robot should be updated with the config.
type ConfigUpdateable interface {
	// Update updates the resource
	Update(context.Context, *config.Config) error
}

func (r *localRobot) updateDefaultServices(ctx context.Context) error {
	// grab all resources
	resources := map[resource.Name]interface{}{}

	for _, n := range r.ResourceNames() {
		// TODO(RSDK-22) if not found, could mean a name clash or a remote service
		res, err := r.ResourceByName(n)
		if err != nil {
			r.Logger().Debugw("not found while grabbing all resources during default svc refresh", "resource", res, "error", err)
		}
		resources[n] = res
	}

	for _, name := range defaultSvc {
		svc, err := r.ResourceByName(name)
		if err != nil {
			return utils.NewResourceNotFoundError(name)
		}
		if updateable, ok := svc.(resource.Updateable); ok {
			if err := updateable.Update(ctx, resources); err != nil {
				return err
			}
		}
		if configUpdateable, ok := svc.(ConfigUpdateable); ok {
			if err := configUpdateable.Update(ctx, r.config); err != nil {
				return err
			}
		}
	}

	for _, svc := range r.internalServices {
		if updateable, ok := svc.(resource.Updateable); ok {
			if err := updateable.Update(ctx, resources); err != nil {
				return err
			}
		}
	}

	return nil
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
func RobotFromConfigPath(ctx context.Context, cfgPath string, logger golog.Logger) (robot.LocalRobot, error) {
	cfg, err := config.Read(ctx, cfgPath, logger)
	if err != nil {
		logger.Fatal("cannot read config")
		return nil, err
	}
	return RobotFromConfig(ctx, cfg, logger)
}

// RobotFromConfig is a helper to process a config and then create a robot based on it.
func RobotFromConfig(ctx context.Context, cfg *config.Config, logger golog.Logger) (robot.LocalRobot, error) {
	tlsConfig := config.NewTLSConfig(cfg)
	processedCfg, err := config.ProcessConfig(cfg, tlsConfig)
	if err != nil {
		return nil, err
	}
	return New(ctx, processedCfg, logger)
}

// RobotFromResources creates a new robot consisting of the given resources. Using RobotFromConfig is preferred
// to support more streamlined reconfiguration functionality.
func RobotFromResources(ctx context.Context, resources map[resource.Name]interface{}, logger golog.Logger) (robot.LocalRobot, error) {
	return newWithResources(ctx, &config.Config{}, resources, logger)
}

// Discover takes a list of subtype and model name pairs and returns corresponding
// component configurations.
func (r *localRobot) Discover(ctx context.Context, qs []discovery.Query) ([]discovery.Discovery, error) {
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
			discoveries = append(discoveries, discovery.Discovery{Query: q, Discovered: discovered})
		}
	}
	return discoveries, nil
}
