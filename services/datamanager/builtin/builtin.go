// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"

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
	logger         logging.Logger
	captureManager *capture.CaptureManager
	syncManager    *sync.SyncManager
}

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (datamanager.Service, error) {
	cm := capture.NewCaptureManager(logger.Sublogger("capture"), clock)
	svc := &builtIn{
		Named:          conf.ResourceName().AsNamed(),
		logger:         logger,
		captureManager: cm,
		syncManager:    sync.NewSyncManager(logger.Sublogger("sync"), clock, cm.FlushCollectors),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return svc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
	svc.captureManager.Close()
	svc.syncManager.Close()
	return nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (svc *builtIn) Sync(ctx context.Context, extra map[string]interface{}) error {
	return svc.syncManager.Sync(ctx, extra)
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
	svcConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	captureConfig := capture.CaptureConfig{
		CaptureDisabled:             svcConfig.CaptureDisabled,
		CaptureDir:                  svcConfig.CaptureDir,
		Tags:                        svcConfig.Tags,
		MaximumCaptureFileSizeBytes: svcConfig.MaximumCaptureFileSizeBytes,
	}
	if err = svc.captureManager.ReconfigureCapture(ctx, deps, conf, captureConfig); err != nil {
		svc.logger.Warnw("DataCapture reconfigure error", "err", err)
		return err
	}

	syncConfig := sync.SyncConfig{
		AdditionalSyncPaths:        svcConfig.AdditionalSyncPaths,
		Tags:                       svcConfig.Tags,
		CaptureDir:                 svc.captureManager.CaptureDir(),
		CaptureDisabled:            svcConfig.CaptureDisabled,
		DeleteEveryNthWhenDiskFull: svcConfig.DeleteEveryNthWhenDiskFull,
		FileLastModifiedMillis:     svcConfig.FileLastModifiedMillis,
		MaximumNumSyncThreads:      svcConfig.MaximumNumSyncThreads,
		ScheduledSyncDisabled:      svcConfig.ScheduledSyncDisabled,
		SelectiveSyncerName:        svcConfig.SelectiveSyncerName,
		SyncIntervalMins:           svcConfig.SyncIntervalMins,
	}
	if err = svc.syncManager.ReconfigureSync(ctx, deps, conf, syncConfig); err != nil {
		svc.logger.Warnw("DataSync reconfigure error", "err", err)
		return err
	}

	g.Success()
	return nil
}
