// Package sync implements datasync for the builtin datamanger
package sync

import (
	"reflect"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/utils"
)

// Config is the sync config from builtin.
type Config struct {
	// AdditionalSyncPaths defines the file system paths
	// that should be synced in addition to the CaptureDir
	// Generally 3rd party programs will write arbitrary
	// files to these directories which are intended to be
	// synced to the cloud by data manager.
	AdditionalSyncPaths []string
	// CaptureDir defines the file system path
	// that data capture will write to and
	// which data sync should sync from.
	CaptureDir string
	// CaptureDisabled, when true disables data capture.
	// sync needs to know if capture is disabled b/c if it
	// is not disabled sync will need to check if
	// the disk has filled up enough that data sync needs to delete
	// capture files without syncing them.
	// See DeleteEveryNthWhenDiskFull for more info.
	// defaults to filepath.Join(os.Getenv("HOME"), ".viam", "capture")
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
	// defaults to 1000
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
	configDisabled := c.ScheduledSyncDisabled || utils.Float64AlmostEqual(c.SyncIntervalMins, 0.0, 0.00001)
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

func (c Config) syncPaths() []string {
	return append([]string{c.CaptureDir}, c.AdditionalSyncPaths...)
}
