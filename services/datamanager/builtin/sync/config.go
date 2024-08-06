package sync

import (
	"reflect"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/utils"
)

// Config is the sync config from builtin.
type Config struct {
	AdditionalSyncPaths        []string
	CaptureDir                 string
	CaptureDisabled            bool
	DeleteEveryNthWhenDiskFull int
	FileLastModifiedMillis     int
	MaximumNumSyncThreads      int
	SyncDisabled               bool
	SelectiveSyncerName        string
	SyncIntervalMins           float64
	Tags                       []string
	SelectiveSyncSensorEnabled bool
	SelectiveSyncSensor        sensor.Sensor
}

func (c Config) schedulerEnabled() bool {
	configDisabled := c.SyncDisabled || utils.Float64AlmostEqual(c.SyncIntervalMins, 0.0, 0.00001)
	selectiveSyncerInvalid := c.SelectiveSyncSensorEnabled && c.SelectiveSyncSensor == nil
	return !configDisabled && !selectiveSyncerInvalid
}

// equal returns true when two Configs are semantically equivalent.
func (c Config) equal(o Config) bool {
	return reflect.DeepEqual(c.AdditionalSyncPaths, o.AdditionalSyncPaths) &&
		c.CaptureDir == o.CaptureDir &&
		c.CaptureDisabled == o.CaptureDisabled &&
		c.DeleteEveryNthWhenDiskFull == o.DeleteEveryNthWhenDiskFull &&
		c.FileLastModifiedMillis == o.FileLastModifiedMillis &&
		c.MaximumNumSyncThreads == o.MaximumNumSyncThreads &&
		c.SyncDisabled == o.SyncDisabled &&
		c.SelectiveSyncerName == o.SelectiveSyncerName &&
		c.SyncIntervalMins == o.SyncIntervalMins &&
		reflect.DeepEqual(c.Tags, o.Tags) &&
		c.SelectiveSyncSensor == o.SelectiveSyncSensor &&
		c.SelectiveSyncSensorEnabled == o.SelectiveSyncSensorEnabled
}

// TODO: Confirm this works for an empty config.
func (c Config) syncPaths() []string {
	return append([]string{c.CaptureDir}, c.AdditionalSyncPaths...)
}
