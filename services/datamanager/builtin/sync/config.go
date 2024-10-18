package sync

import (
	"reflect"
	"strings"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
)

// Config is the sync config from builtin.
type Config struct {
	Flag bool
	// AdditionalSyncPaths defines the file system paths
	// that should be synced in addition to the CaptureDir.
	// Generally 3rd party programs will write arbitrary
	// files to these directories which are intended to be
	// synced to the cloud by data manager.
	AdditionalSyncPaths []string
	// CaptureDir defines the file system path
	// that data capture will write to and
	// which data sync should sync from.
	// defaults to filepath.Join(os.Getenv("HOME"), ".viam", "capture")
	CaptureDir string
	// CaptureDisabled, when true disables data capture.
	// sync needs to know if capture is disabled b/c if it
	// is not disabled sync will need to check if
	// the disk has filled up enough that data sync needs to delete
	// capture files without syncing them.
	// See DeleteEveryNthWhenDiskFull for more info.
	CaptureDisabled bool
	// DeleteEveryNthWhenDiskFull defines the `n` in
	// the psudocode:
	// ```go
	// captureEnabled := !config.CaptureDisabled
	// if captureEnabled {
	//     for {
	//         time.Sleep(time.Second*30)
	//         if diskFull() {
	//             for i, file in range dataCaptureDirFiles {
	//                 if fileIndex % n == 0 {
	//                     delete file
	//                 }
	//             }
	//         }
	//     }
	// }
	// ```
	//
	// which in english reads:
	//
	// If datacapture is enabled then every 30 seconds
	// if the disk is full (which is defined as the disk
	// is 90% full and 50% is contributed by the CaptureDir)
	// delete every Nth file in the CaptureDir & child
	// directories.
	//
	// The intent is to prevent data capture from filling up the
	// disk if the robot is unable to sync data for a long period
	// of time. Defaults to 5.
	DeleteEveryNthWhenDiskFull int
	// FileLastModifiedMillis defines the number of milliseconds that
	// we should wait for an arbitrary file (aka a file that doesn't end in
	// either the .prog nor the .capture file extension) before we consider
	// it as being ready to sync.
	// defaults to 10000, aka 10 seconds
	FileLastModifiedMillis int
	// MaximumNumSyncThreads defines the maximum number of goroutines which
	// data sync should create to sync data to the cloud
	// defaults to runtime.NumCpu() / 2
	MaximumNumSyncThreads int
	// ScheduledSyncDisabled, when true disables data capture syncing every SyncIntervalMins
	ScheduledSyncDisabled bool
	// SelectiveSyncerName defines the name of the selective sync sensor. Ignored when empty string
	SelectiveSyncerName string
	// SyncIntervalMins defines interval in minutes that scheduled sync should run. Ignored if
	// ScheduledSyncDisabled is true
	SyncIntervalMins float64
	// Tags defines the tags which should be applied to arbitrary files at sync time
	Tags []string
	// SelectiveSyncSensorEnabled when set to true, indicates that SelectiveSyncerName was non empty string
	// (meaning that the user configured a sync sensor). This will cause data sync to NOT sync if
	// SelectiveSyncSensor is nil (which will happen if resource graph doesn't have a resource with that name)
	SelectiveSyncSensorEnabled bool
	// SelectiveSyncSensor the selective sync sensor, which if non nil, will cause scheduled sync to NOT sync
	// unil the Readings method of the SelectiveSyncSensor (when called on the SyncIntervalMins interval) returns
	// the a key of datamanager.ShouldSyncKey and a value of `true`
	SelectiveSyncSensor sensor.Sensor
}

func (c Config) schedulerEnabled() bool {
	configDisabled := c.ScheduledSyncDisabled
	selectiveSyncerInvalid := c.SelectiveSyncSensorEnabled && c.SelectiveSyncSensor == nil
	return !configDisabled && !selectiveSyncerInvalid
}

// Equal returns true when two Configs are semantically equivalent.
func (c Config) Equal(o Config) bool {
	return reflect.DeepEqual(c.AdditionalSyncPaths, o.AdditionalSyncPaths) &&
		c.CaptureDir == o.CaptureDir &&
		c.CaptureDisabled == o.CaptureDisabled &&
		c.DeleteEveryNthWhenDiskFull == o.DeleteEveryNthWhenDiskFull &&
		c.FileLastModifiedMillis == o.FileLastModifiedMillis &&
		c.MaximumNumSyncThreads == o.MaximumNumSyncThreads &&
		c.ScheduledSyncDisabled == o.ScheduledSyncDisabled &&
		c.SelectiveSyncerName == o.SelectiveSyncerName &&
		c.SyncIntervalMins == o.SyncIntervalMins &&
		reflect.DeepEqual(c.Tags, o.Tags) &&
		c.SelectiveSyncSensorEnabled == o.SelectiveSyncSensorEnabled &&
		c.SelectiveSyncSensor == o.SelectiveSyncSensor
}

func (c *Config) logDiff(o Config, logger logging.Logger) {
	if c.Equal(o) {
		return
	}

	logger.Info("sync config changes:")
	if !reflect.DeepEqual(c.AdditionalSyncPaths, o.AdditionalSyncPaths) {
		logger.Infof("additional_sync_paths: old: %s, new: %s",
			strings.Join(c.AdditionalSyncPaths, " "), strings.Join(o.AdditionalSyncPaths, " "))
	}

	if c.CaptureDir != o.CaptureDir {
		logger.Infof("capture_dir: old: %s, new: %s", c.CaptureDir, o.CaptureDir)
	}

	if c.CaptureDisabled != o.CaptureDisabled {
		logger.Infof("capture_disabled: old: %t, new: %t", c.CaptureDisabled, o.CaptureDisabled)
	}

	if c.DeleteEveryNthWhenDiskFull != o.DeleteEveryNthWhenDiskFull {
		logger.Infof("delete_every_nth_when_disk_full: old: %d, new: %d",
			c.DeleteEveryNthWhenDiskFull, o.DeleteEveryNthWhenDiskFull)
	}

	if c.FileLastModifiedMillis != o.FileLastModifiedMillis {
		logger.Infof("file_last_modified_millis: old: %d, new: %d", c.FileLastModifiedMillis, o.FileLastModifiedMillis)
	}

	if c.MaximumNumSyncThreads != o.MaximumNumSyncThreads {
		logger.Infof("maximum_num_sync_threads: old: %d, new: %d", c.MaximumNumSyncThreads, o.MaximumNumSyncThreads)
	}

	if c.ScheduledSyncDisabled != o.ScheduledSyncDisabled {
		logger.Infof("sync_disabled: old: %t, new: %t", c.ScheduledSyncDisabled, o.ScheduledSyncDisabled)
	}

	if c.SelectiveSyncerName != o.SelectiveSyncerName {
		logger.Infof("selective_syncer_name: old: %s, new: %s", c.SelectiveSyncerName, o.SelectiveSyncerName)
	}

	if c.SyncIntervalMins != o.SyncIntervalMins {
		logger.Infof("sync_interval_mins: old: %f, new: %f", c.SyncIntervalMins, o.SyncIntervalMins)
	}

	if !reflect.DeepEqual(c.Tags, o.Tags) {
		logger.Infof("tags: old: %s, new: %s", strings.Join(c.Tags, " "), strings.Join(o.Tags, " "))
	}

	if c.SelectiveSyncSensorEnabled != o.SelectiveSyncSensorEnabled {
		logger.Infof("SelectiveSyncSensorEnabled: old: %t, new: %t", c.SelectiveSyncSensorEnabled, o.SelectiveSyncSensorEnabled)
	}

	if c.SelectiveSyncSensor != o.SelectiveSyncSensor {
		oldName := ""
		if c.SelectiveSyncSensor != nil {
			oldName = c.SelectiveSyncSensor.Name().String()
		}

		newName := ""
		if o.SelectiveSyncSensor != nil {
			newName = o.SelectiveSyncSensor.Name().String()
		}
		logger.Infof("SelectiveSyncSensor: old: %s, new: %s", oldName, newName)
	}
}

// SyncPaths returns the capture directory and additional sync paths as a slice.
func (c Config) SyncPaths() []string {
	return append([]string{c.CaptureDir}, c.AdditionalSyncPaths...)
}
