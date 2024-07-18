// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	clk "github.com/benbjohnson/clock"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin/capture"
	"go.viam.com/rdk/services/datamanager/builtin/sync"
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

var clock = clk.New()

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

// builtIn initializes and orchestrates data capture collectors for registered component/methods.
type builtIn struct {
	resource.Named
	logger  logging.Logger
	capture *capture.Capture
	sync    *sync.Sync
}

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (datamanager.Service, error) {
	cm := capture.NewManager(logger.Sublogger("capture"), clock)
	svc := &builtIn{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger,
		capture: cm,
		sync:    sync.NewSync(logger.Sublogger("sync"), clock, cm.FlushCollectors),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return svc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
	svc.capture.Close()
	svc.sync.Close()
	return nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (svc *builtIn) Sync(ctx context.Context, extra map[string]interface{}) error {
	return svc.sync.Sync(ctx, extra)
}

// Reconfigure updates the data manager service when the config has changed.
func (svc *builtIn) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	// TODO: Move this into each of captureManger and syncManager
	g := utils.NewGuard(func() { goutils.UncheckedError(svc.Close(ctx)) })
	defer g.OnFail()
	c, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if !utils.IsTrustedEnvironment(ctx) && c.CaptureDir != "" && c.CaptureDir != viamCaptureDotDir {
		return ErrCaptureDirectoryConfigurationDisabled
	}

	cloudConnSvc, err := resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		// If this error occurs it's a resource graph error
		return err
	}

	captureDir := viamCaptureDotDir
	if c.CaptureDir != "" {
		captureDir = c.CaptureDir
	}

	svc.sync.Reconfigure(ctx, deps, conf, syncConfig(c, captureDir), cloudConnSvc)

	if err := svc.capture.Reconfigure(ctx, deps, conf, captureConfig(c, captureDir)); err != nil {
		svc.logger.Warnw("DataCapture reconfigure error", "err", err)
		return err
	}

	g.Success()
	return nil
}

func captureConfig(c *Config, captureDir string) capture.Config {
	return capture.Config{
		CaptureDisabled:             c.CaptureDisabled,
		CaptureDir:                  captureDir,
		Tags:                        c.Tags,
		MaximumCaptureFileSizeBytes: c.MaximumCaptureFileSizeBytes,
	}
}

func syncConfig(c *Config, captureDir string) sync.Config {
	return sync.Config{
		AdditionalSyncPaths:        c.AdditionalSyncPaths,
		Tags:                       c.Tags,
		CaptureDir:                 captureDir,
		CaptureDisabled:            c.CaptureDisabled,
		DeleteEveryNthWhenDiskFull: c.DeleteEveryNthWhenDiskFull,
		FileLastModifiedMillis:     c.FileLastModifiedMillis,
		MaximumNumSyncThreads:      c.MaximumNumSyncThreads,
		ScheduledSyncDisabled:      c.ScheduledSyncDisabled,
		SelectiveSyncerName:        c.SelectiveSyncerName,
		SyncIntervalMins:           c.SyncIntervalMins,
	}
}
