package builtin

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/builtin/capture"
	"go.viam.com/rdk/services/datamanager/builtin/shared"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/utils"
)

// Sync defaults.
const (
	// Default time to wait in milliseconds to check if a file has been modified.
	defaultFileLastModifiedMillis = 10000.0
	// defaultDeleteEveryNth temporarily public for tests.
	// defaultDeleteEveryNth configures the N in the following expression `captureFileIndex % N == 0`
	// which is evaluated if the file deletion threshold has been reached. If `captureFileIndex % N == 0`
	// return true then the file will be deleted to free up space.
	defaultDeleteEveryNth = 5
	// defaultDiskUsageThreshold and defaultCaptureDirThreshold are the thresholds at which file deletion might occur.
	// If disk usage is at or above this threshold, AND the capture directory makes up at least CaptureDirThreshold (%) of the disk usage,
	// then file deletion will occur. If disk usage is at or above the disk usage threshold, but the capture directory is
	// below the capture directory threshold, then file deletion will not occur but a warning will be logged periodically.
	defaultDiskUsageThreshold  = 0.9
	defaultCaptureDirThreshold = 0.5
	// defaultSyncIntervalMins is the sync interval that will be set if the config's sync_interval_mins is zero (including when it is unset).
	defaultSyncIntervalMins = 0.1
	// syncIntervalMinsEpsilon is the value below which SyncIntervalMins is considered zero.
	syncIntervalMinsEpsilon = 0.0001
)

// Capture Defaults
// Default maximum size in bytes of a data capture file.
var defaultMaxCaptureSize = int64(256 * 1024)

// Config describes how to configure the service.
// See sync.Config and capture.Config for docs on what each field does
// to both sync & capture respectively.
type Config struct {
	// Sync & Capture
	CaptureDir string   `json:"capture_dir"`
	Tags       []string `json:"tags"`
	// Capture
	CaptureDisabled    bool                 `json:"capture_disabled"`
	MongoCaptureConfig *capture.MongoConfig `json:"mongo_capture_config"`
	// File Deletion Parameters
	DeleteEveryNthWhenDiskFull  int     `json:"delete_every_nth_when_disk_full"`
	MaximumCaptureFileSizeBytes int64   `json:"maximum_capture_file_size_bytes"`
	DiskUsageDeletionThreshold  float64 `json:"disk_usage_deletion_threshold"`
	CaptureDirDeletionThreshold float64 `json:"capture_dir_deletion_threshold"`
	// Sync
	AdditionalSyncPaths    []string `json:"additional_sync_paths"`
	FileLastModifiedMillis int      `json:"file_last_modified_millis"`
	MaximumNumSyncThreads  int      `json:"maximum_num_sync_threads"`
	ScheduledSyncDisabled  bool     `json:"sync_disabled"`
	SelectiveSyncerName    string   `json:"selective_syncer_name"`
	SyncIntervalMins       float64  `json:"sync_interval_mins"`
}

// Validate returns components which will be depended upon weakly due to the above matcher.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if c.SyncIntervalMins < 0 {
		return nil, nil, errors.New("sync_interval_mins can't be negative")
	}
	if c.MaximumNumSyncThreads < 0 {
		return nil, nil, errors.New("maximum_num_sync_threads can't be negative")
	}
	if c.FileLastModifiedMillis < 0 {
		return nil, nil, errors.New("file_last_modified_millis can't be negative")
	}
	if c.MaximumCaptureFileSizeBytes < 0 {
		return nil, nil, errors.New("maximum_capture_file_size_bytes can't be negative")
	}
	if c.DeleteEveryNthWhenDiskFull < 0 {
		return nil, nil, errors.New("delete_every_nth_when_disk_full can't be negative")
	}
	if c.DiskUsageDeletionThreshold < 0 {
		return nil, nil, errors.New("disk_usage_deletion_threshold can't be negative")
	}
	if c.CaptureDirDeletionThreshold < 0 {
		return nil, nil, errors.New("capture_dir_deletion_threshold can't be negative")
	}
	return []string{cloud.InternalServiceName.String()}, nil, nil
}

func (c *Config) getCaptureDir(logger logging.Logger) string {
	captureDir := shared.ViamCaptureDotDir
	if c.CaptureDir != "" {
		captureDir = c.CaptureDir
		if strings.HasPrefix(captureDir, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				logger.Warn("failed to get user home directory")
				return captureDir
			}
			return filepath.Join(home, captureDir[1:])
		}
	}
	return captureDir
}

func (c *Config) captureConfig(logger logging.Logger) capture.Config {
	maximumCaptureFileSizeBytes := defaultMaxCaptureSize
	if c.MaximumCaptureFileSizeBytes != 0 {
		maximumCaptureFileSizeBytes = c.MaximumCaptureFileSizeBytes
	}
	return capture.Config{
		CaptureDisabled:             c.CaptureDisabled,
		CaptureDir:                  c.getCaptureDir(logger),
		Tags:                        c.Tags,
		MaximumCaptureFileSizeBytes: maximumCaptureFileSizeBytes,
		MongoConfig:                 c.MongoCaptureConfig,
	}
}

func (c *Config) syncConfig(syncSensor sensor.Sensor, syncSensorEnabled bool, logger logging.Logger) datasync.Config {
	newMaxSyncThreadValue := runtime.NumCPU() / 2
	if c.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = c.MaximumNumSyncThreads
	}
	c.MaximumNumSyncThreads = newMaxSyncThreadValue

	deleteEveryNthValue := defaultDeleteEveryNth
	if c.DeleteEveryNthWhenDiskFull != 0 {
		deleteEveryNthValue = c.DeleteEveryNthWhenDiskFull
	}
	c.DeleteEveryNthWhenDiskFull = deleteEveryNthValue

	diskUsageThreshold := defaultDiskUsageThreshold
	if c.DiskUsageDeletionThreshold != 0 {
		diskUsageThreshold = c.DiskUsageDeletionThreshold
	}
	c.DiskUsageDeletionThreshold = diskUsageThreshold

	captureDirThreshold := defaultCaptureDirThreshold
	if c.CaptureDirDeletionThreshold != 0 {
		captureDirThreshold = c.CaptureDirDeletionThreshold
	}
	c.CaptureDirDeletionThreshold = captureDirThreshold

	fileLastModifiedMillis := c.FileLastModifiedMillis
	if fileLastModifiedMillis <= 0 {
		fileLastModifiedMillis = defaultFileLastModifiedMillis
	}
	c.FileLastModifiedMillis = fileLastModifiedMillis

	syncIntervalMins := c.SyncIntervalMins
	if utils.Float64AlmostEqual(c.SyncIntervalMins, 0, syncIntervalMinsEpsilon) {
		syncIntervalMins = defaultSyncIntervalMins
		logger.Infof("sync_interval_mins set to %f which is below allowed minimum of %f, setting to default of %f",
			c.SyncIntervalMins, syncIntervalMinsEpsilon, defaultSyncIntervalMins)
	}

	return datasync.Config{
		AdditionalSyncPaths:         c.AdditionalSyncPaths,
		Tags:                        c.Tags,
		CaptureDir:                  c.getCaptureDir(logger),
		CaptureDisabled:             c.CaptureDisabled,
		DeleteEveryNthWhenDiskFull:  c.DeleteEveryNthWhenDiskFull,
		DiskUsageDeletionThreshold:  c.DiskUsageDeletionThreshold,
		CaptureDirDeletionThreshold: c.CaptureDirDeletionThreshold,
		FileLastModifiedMillis:      c.FileLastModifiedMillis,
		MaximumNumSyncThreads:       c.MaximumNumSyncThreads,
		ScheduledSyncDisabled:       c.ScheduledSyncDisabled,
		SelectiveSyncerName:         c.SelectiveSyncerName,
		SyncIntervalMins:            syncIntervalMins,
		SelectiveSyncSensor:         syncSensor,
		SelectiveSyncSensorEnabled:  syncSensorEnabled,
	}
}
