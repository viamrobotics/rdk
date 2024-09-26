// Package builtin captures data from a robot's components, persists the captured data to disk and sync it to the cloud
// when possible.
package builtin

// Design note:
// Builtin is a thin wrapper around builtin.capture and builtin.sync packages which manage data capture and data sync (respectively).
// Builtin can operate with either sync, capture, both or neither enabled.
// The main responsibility of the builtin package is to collect the dependencies required by sync and capture from resource graph
// and to provide a thread safe interface for resource graph to call into the datasync and data capture during initialization,
// reconfiguration, and shutdown.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/utils/rpc"
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

var (
	// ErrCaptureDirectoryConfigurationDisabled happens when the viam-server is run with
	// `-untrusted-env` and the capture directory is not `~/.viam/capture`.
	ErrCaptureDirectoryConfigurationDisabled = errors.New("changing the capture directory is prohibited in this environment")
	viamCaptureDotDir                        = filepath.Join(os.Getenv("HOME"), ".viam", "capture")
	// This clock only exists for tests.
	// At time of writing only a single test depends on it.
	// We should endevor to not add more tests that depend on it unless absolutiely necessary.
	clk = clock.New()
	// diskSummaryLogInterval is the frequency a summary of the sync paths are logged.
	diskSummaryLogInterval = time.Minute
)

// In order for a collector to be captured by Data Capture, it must be included as a weak dependency.
func init() {
	constructor := func(
		ctx context.Context,
		deps resource.Dependencies,
		conf resource.Config,
		logger logging.Logger,
	) (datamanager.Service, error) {
		// we inject v1.NewDataSyncServiceClient and datasync.ConnToConnectivityStateEnabled as dependencies for tests
		return New(
			ctx,
			deps,
			conf,
			v1.NewDataSyncServiceClient,
			datasync.ConnToConnectivityState,
			logger,
		)
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

// builtIn initializes and orchestrates data capture and data sync based on the config.
type builtIn struct {
	resource.Named
	logger logging.Logger

	mu                sync.Mutex
	capture           *capture.Capture
	sync              *datasync.Sync
	diskSummaryLogger *diskSummaryLogger
}

// New returns a new builtin data manager service for the given robot.
func New(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	cloudClientConstructor func(grpc.ClientConnInterface) v1.DataSyncServiceClient,
	connToConnectivityStateEnabled func(conn rpc.ClientConn) datasync.ConnectivityState,
	logger logging.Logger,
) (datamanager.Service, error) {
	logger.Info("New START")
	defer logger.Info("New END")
	capture := capture.New(
		clk,
		logger.Sublogger("capture"),
	)
	// sync needs to be able to flush collectors so that in memory data can be flushed to disk before a given sync interval
	// or manual sync call
	sync := datasync.New(
		cloudClientConstructor,
		connToConnectivityStateEnabled,
		capture.FlushCollectors,
		clk,
		logger.Sublogger("sync"),
	)
	diskSummaryLogger := newDiskSummaryLogger(logger)
	svc := &builtIn{
		Named:             conf.ResourceName().AsNamed(),
		logger:            logger,
		capture:           capture,
		sync:              sync,
		diskSummaryLogger: diskSummaryLogger,
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return svc, nil
}

// Close releases all resources managed by data_manager.
func (b *builtIn) Close(_ context.Context) error {
	b.logger.Info("Close START")
	defer b.logger.Info("Close END")
	b.mu.Lock()
	defer b.mu.Unlock()
	b.diskSummaryLogger.close()
	b.capture.Close()
	b.sync.Close()
	return nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (b *builtIn) Sync(ctx context.Context, extra map[string]interface{}) error {
	b.logger.Info("Sync START")
	defer b.logger.Info("Sync END")
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sync.Sync(ctx, extra)
}

// Reconfigure updates the data manager service when the config has changed.
// At time of writing Reconfigure only returns an error in one of the following unrecoverable error cases:
//  1. There is some static (aka compile time) error which we currently are only able to detected at runtime:
//     a. Config isn't a NativeConfig,
//     b. resource graph didn't boot the internal cloud service before booting datamanager (which would be a resource graph bug)
//     c. the resource.Config.AssociatedAttributes were not of type *datamanager.AssociatedConfig
//     (which would be a bug in resource graph or in the collector framework code in resource graph)
//
// 2. The user is running data manager in an untrusted env (see the comment above ErrCaptureDirectoryConfigurationDisabled
// for more details) and has specified a non default capture directory.
//
// The only time long lived resources (aka goroutines) are booted is in the calls to capture.Reconfigure and sync.Reconfigure
// It is important that we only call those methods after we have checked for all errors, otherwise we could leak resources
// when errors occur.
// If an error occurs after the first Reconfigure call, data capture & data sync will continue to function using the old config
// until a successful Reconfigure call is made or Close is called.
func (b *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	b.logger.Info("Reconfigure START")
	defer b.logger.Info("Reconfigure END")
	c, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		// If this error occurs it is due to the builtin.Config not being a native config which is a
		// static error that could only be introduced at compile time.
		return err
	}

	if !utils.IsTrustedEnvironment(ctx) && c.CaptureDir != "" && c.CaptureDir != viamCaptureDotDir {
		// see comment above this error definition for when this happens
		return ErrCaptureDirectoryConfigurationDisabled
	}

	cloudConnSvc, err := resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		// If this error occurs it's a resource graph error
		return err
	}

	captureConfig := c.captureConfig()
	collectorConfigsByResource, err := lookupCollectorConfigsByResource(deps, conf, captureConfig.CaptureDir, b.logger)
	if err != nil {
		// If this error occurs it's a resource graph error
		return err
	}

	syncSensor, syncSensorEnabled := syncSensorFromDeps(c.SelectiveSyncerName, deps, b.logger)
	syncConfig := c.syncConfig(syncSensor, syncSensorEnabled, b.logger)

	b.mu.Lock()
	defer b.mu.Unlock()
	// These Reconfigure calls are the only methods in builtin.Reconfigure which create / destroy resources.
	// It is important that no errors happen for a given Reconfigure call after we being callin Reconfigure on capture & sync
	// or we could leak goroutines, wasting resources and cauing bugs due to duplicate work.
	b.diskSummaryLogger.reconfigure(syncConfig.SyncPaths(), diskSummaryLogInterval)
	b.capture.Reconfigure(ctx, collectorConfigsByResource, captureConfig)
	b.sync.Reconfigure(ctx, syncConfig, cloudConnSvc)

	return nil
}

func syncSensorFromDeps(name string, deps resource.Dependencies, logger logging.Logger) (sensor.Sensor, bool) {
	if name == "" {
		return nil, false
	}
	syncSensor, err := sensor.FromDependencies(deps, name)
	if err != nil {
		// see sync.Config for how this affects whether or not scheduled sync will run
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
) (capture.CollectorConfigsByResource, error) {
	collectorConfigsByResource := capture.CollectorConfigsByResource{}
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
