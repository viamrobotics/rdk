// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"

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
	resource.RegisterService(
		datamanager.API,
		resource.DefaultServiceModel,
		resource.Registration[datamanager.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: slam.SubtypeName},
				resource.SubtypeMatcher{Subtype: vision.SubtypeName},
			},
		})
}

// Config describes how to configure the service.
type Config struct {
	// Sync & Capture
	CaptureDir string   `json:"capture_dir"`
	Tags       []string `json:"tags"`
	// Capture
	CaptureDisabled             bool  `json:"capture_disabled"`
	DeleteEveryNthWhenDiskFull  int   `json:"delete_every_nth_when_disk_full"`
	MaximumCaptureFileSizeBytes int64 `json:"maximum_capture_file_size_bytes"`
	// Sync
	AdditionalSyncPaths    []string `json:"additional_sync_paths"`
	FileLastModifiedMillis int      `json:"file_last_modified_millis"`
	MaximumNumSyncThreads  int      `json:"maximum_num_sync_threads"`
	ScheduledSyncDisabled  bool     `json:"sync_disabled"`
	SelectiveSyncerName    string   `json:"selective_syncer_name"`
	SyncIntervalMins       float64  `json:"sync_interval_mins"`
}

// Validate returns components which will be depended upon weakly due to the above matcher.
func (c *Config) Validate(path string) ([]string, error) {
	return []string{cloud.InternalServiceName.String()}, nil
}

func (c *Config) getCaptureDir() string {
	captureDir := viamCaptureDotDir
	if c.CaptureDir != "" {
		captureDir = c.CaptureDir
	}
	return captureDir
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
	logger logging.Logger,
) (datamanager.Service, error) {
	capture := capture.New(logger.Sublogger("capture"))
	sync := datasync.New(
		v1.NewDataSyncServiceClient,
		capture.FlushCollectors,
		logger.Sublogger("sync"),
	)
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
func (b *builtIn) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
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

	syncSensor := syncSensorFromDeps(c.SelectiveSyncerName, deps, b.logger)
	b.sync.Reconfigure(ctx, syncConfig(c, syncSensor), cloudConnSvc)
	err = b.capture.Reconfigure(ctx, deps, conf, captureConfig(c))
	if err != nil {
		// If this error occurs it's a resource graph error
		return err
	}
	g.Success()

	return nil
}

func captureConfig(c *Config) capture.Config {
	return capture.Config{
		CaptureDisabled:             c.CaptureDisabled,
		CaptureDir:                  c.getCaptureDir(),
		Tags:                        c.Tags,
		MaximumCaptureFileSizeBytes: c.MaximumCaptureFileSizeBytes,
	}
}

const (
	// Default time to wait in milliseconds to check if a file has been modified.
	defaultFileLastModifiedMillis = 10000.0
	// DefaultMaxParallelSyncRoutines is the maximum number of sync goroutines that can be running at once.
	DefaultMaxParallelSyncRoutines = 100
	// DefaultDeleteEveryNth temporarily public for tests.
	DefaultDeleteEveryNth = 5
)

func syncConfig(c *Config, syncSensor sensor.Sensor) datasync.Config {
	newMaxSyncThreadValue := DefaultMaxParallelSyncRoutines
	if c.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = c.MaximumNumSyncThreads
	}
	c.MaximumNumSyncThreads = newMaxSyncThreadValue

	deleteEveryNthValue := DefaultDeleteEveryNth
	if c.DeleteEveryNthWhenDiskFull != 0 {
		deleteEveryNthValue = c.DeleteEveryNthWhenDiskFull
	}
	c.DeleteEveryNthWhenDiskFull = deleteEveryNthValue

	fileLastModifiedMillis := c.FileLastModifiedMillis
	if fileLastModifiedMillis <= 0 {
		fileLastModifiedMillis = defaultFileLastModifiedMillis
	}
	c.FileLastModifiedMillis = fileLastModifiedMillis
	return datasync.Config{
		AdditionalSyncPaths:        c.AdditionalSyncPaths,
		Tags:                       c.Tags,
		CaptureDir:                 c.getCaptureDir(),
		CaptureDisabled:            c.CaptureDisabled,
		DeleteEveryNthWhenDiskFull: c.DeleteEveryNthWhenDiskFull,
		FileLastModifiedMillis:     c.FileLastModifiedMillis,
		MaximumNumSyncThreads:      c.MaximumNumSyncThreads,
		SyncDisabled:               c.ScheduledSyncDisabled,
		SelectiveSyncerName:        c.SelectiveSyncerName,
		SyncIntervalMins:           c.SyncIntervalMins,
		SyncSensor:                 syncSensor,
	}
}

func syncSensorFromDeps(selectiveSyncerName string, deps resource.Dependencies, logger logging.Logger) sensor.Sensor {
	var syncSensor sensor.Sensor
	if selectiveSyncerName != "" {
		tmp, err := sensor.FromDependencies(deps, selectiveSyncerName)
		if err != nil {
			logger.Errorw(
				"unable to initialize selective syncer; will not sync at all until fixed or removed from config", "error", err.Error())
		}
		syncSensor = tmp
	}
	return syncSensor
}
