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
	SyncSensor                 sensor.Sensor
}

func (c Config) disabled() bool {
	return c.SyncDisabled || utils.Float64AlmostEqual(c.SyncIntervalMins, 0.0, 0.00001)
}

// Equal returns true when two Configs are semantically equivalent.
func (c Config) Equal(o Config) bool {
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
		c.SyncSensor == o.SyncSensor
}

// TODO: Confirm this works for an empty config.
func (c Config) syncPaths() []string {
	return append([]string{c.CaptureDir}, c.AdditionalSyncPaths...)
}
