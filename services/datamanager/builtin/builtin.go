// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin/capture"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

// ErrCaptureDirectoryConfigurationDisabled happens when the viam-server is run with
// `-untrusted-env` and the capture directory is not `~/.viam`.
var (
	ErrCaptureDirectoryConfigurationDisabled = errors.New("changing the capture directory is prohibited in this environment")
	viamCaptureDotDir                        = filepath.Join(os.Getenv("HOME"), ".viam", "capture")
)

// In order for a collector to be captured by Data Capture, it must be included as a weak dependency.
func init() {
	constructor := func(
		ctx context.Context,
		deps resource.Dependencies,
		conf resource.Config,
		logger logging.Logger,
	) (datamanager.Service, error) {
		return NewBuiltIn(ctx, deps, conf, v1.NewDataSyncServiceClient, logger)
	}
	resource.RegisterService(
		datamanager.API,
		resource.DefaultServiceModel,
		resource.Registration[datamanager.Service, *Config]{
			Constructor: constructor,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: slam.SubtypeName},
				resource.SubtypeMatcher{Subtype: vision.SubtypeName},
			},
		})
}

// builtIn initializes and orchestrates data capture collectors for registered component/methods.
type builtIn struct {
	resource.Named
	logger logging.Logger

	mu      sync.Mutex
	capture *capture.Capture
	sync    *datasync.Sync
}

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	cloudClientConstructor func(grpc.ClientConnInterface) v1.DataSyncServiceClient,
	logger logging.Logger,
) (datamanager.Service, error) {
	capture := capture.New(logger.Sublogger("capture"))
	sync := datasync.New(cloudClientConstructor, capture.FlushCollectors, logger.Sublogger("sync"))
	svc := &builtIn{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger,
		capture: capture,
		sync:    sync,
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return svc, nil
}

// Close releases all resources managed by data_manager.
func (b *builtIn) Close(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.close()
	return nil
}

func (b *builtIn) close() {
	b.capture.Close()
	b.sync.Close()
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (b *builtIn) Sync(ctx context.Context, extra map[string]interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sync.Sync(ctx, extra)
}

// Reconfigure updates the data manager service when the config has changed.
func (b *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	var err error
	failFunc := func() {
		b.logger.Errorf("unrecoverable datamanager Reconfigure error: %s", err.Error())
		b.close()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	g := utils.NewGuard(failFunc)
	defer g.OnFail()
	var c *Config
	c, err = resource.NativeConfig[*Config](conf)
	if err != nil {
		// If this error occurs it is due to the builtin.Config not being a native config which is a
		// static error that could only be introduced at compile time.
		return err
	}

	if !utils.IsTrustedEnvironment(ctx) && c.CaptureDir != "" && c.CaptureDir != viamCaptureDotDir {
		err = ErrCaptureDirectoryConfigurationDisabled
		return err
	}

	var cloudConnSvc cloud.ConnectionService
	cloudConnSvc, err = resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		// If this error occurs it's a resource graph error
		return err
	}

	captureConfig := captureConfig(c)

	var collectorConfigsByResource map[resource.Resource][]datamanager.DataCaptureConfig
	collectorConfigsByResource, err = lookupCollectorConfigsByResource(deps, conf, captureConfig.CaptureDir, b.logger)
	if err != nil {
		// If this error occurs it's a resource graph error
		return err
	}

	b.capture.Reconfigure(ctx, collectorConfigsByResource, captureConfig)
	syncSensor, syncSensorEnabled := syncSensorFromDeps(c.SelectiveSyncerName, deps, b.logger)
	b.sync.Reconfigure(ctx, syncConfig(c, syncSensor, syncSensorEnabled), cloudConnSvc)
	g.Success()

	return nil
}

func syncSensorFromDeps(
	selectiveSyncerName string,
	deps resource.Dependencies,
	logger logging.Logger,
) (sensor.Sensor, bool) {
	if selectiveSyncerName == "" {
		return nil, false
	}
	syncSensor, err := sensor.FromDependencies(deps, selectiveSyncerName)
	if err != nil {
		// see sync.Config.schedulerEnabled() for how this affects whether or not scheduled sync will run
		logger.Errorw(
			"unable to initialize selective syncer; will not schedule sync at all until fixed or removed from config", "error", err.Error())
		return nil, true
	}
	return syncSensor, true
}

// Lookup the collector configs associated with the data manager service.
func lookupCollectorConfigsByResource(
	deps resource.Dependencies,
	resConfig resource.Config,
	captureDir string,
	logger logging.Logger,
) (map[resource.Resource][]datamanager.DataCaptureConfig, error) {
	collectorConfigsByResource := map[resource.Resource][]datamanager.DataCaptureConfig{}
	for name, rawAssocCfg := range resConfig.AssociatedAttributes {
		assocCfg, err := utils.AssertType[*datamanager.AssociatedConfig](rawAssocCfg)
		if err != nil {
			// This would only happen if there is a bug in resource graph
			return nil, err
		}
		res, err := deps.Lookup(name)
		if err != nil {
			logger.Warnw("datamanager failed to lookup resource from config", "error", err)
			continue
		}

		collectorConfigs := []datamanager.DataCaptureConfig{}
		for _, collectorConfig := range assocCfg.CaptureMethods {
			// we need to set the CaptureDirectory to that in the data manager config
			collectorConfig.CaptureDirectory = captureDir
			collectorConfigs = append(collectorConfigs, collectorConfig)
		}
		collectorConfigsByResource[res] = collectorConfigs
	}
	return collectorConfigsByResource, nil
}
