package builtin

import (
	"runtime"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/services/datamanager/builtin/capture"
	"go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/testutils/inject"
)

var fullConfig = &Config{
	AdditionalSyncPaths:         []string{"/tmp/a", "/tmp/b"},
	CaptureDir:                  "/tmp/some/path",
	CaptureDisabled:             true,
	DeleteEveryNthWhenDiskFull:  2,
	FileLastModifiedMillis:      50000,
	MaximumCaptureFileSizeBytes: 5,
	MaximumNumSyncThreads:       10,
	ScheduledSyncDisabled:       true,
	SelectiveSyncerName:         "some name",
	SyncIntervalMins:            0.5,
	Tags:                        []string{"a", "b", "c"},
}

func TestConfig(t *testing.T) {
	t.Run("Validate returns the internal cloud service name", func(t *testing.T) {
		c := &Config{}
		deps, err := c.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{cloud.InternalServiceName.String()})
	})

	t.Run("getCaptureDir", func(t *testing.T) {
		t.Run("returns the default capture directory by default", func(t *testing.T) {
			c := &Config{}
			test.That(t, c.getCaptureDir(), test.ShouldResemble, viamCaptureDotDir)
		})

		t.Run("returns CaptureDir if set", func(t *testing.T) {
			c := &Config{CaptureDir: "/tmp/some/path"}
			test.That(t, c.getCaptureDir(), test.ShouldResemble, "/tmp/some/path")
		})
	})

	t.Run("captureConfig())", func(t *testing.T) {
		t.Run("returns a capture config with defaults when called on an empty config", func(t *testing.T) {
			c := &Config{}
			test.That(t, c.captureConfig(), test.ShouldResemble, capture.Config{
				CaptureDir:                  viamCaptureDotDir,
				MaximumCaptureFileSizeBytes: defaultMaxCaptureSize,
			})
		})

		t.Run("returns a capture config with overridden defaults when called on a full config", func(t *testing.T) {
			test.That(t, fullConfig.captureConfig(), test.ShouldResemble, capture.Config{
				CaptureDisabled:             true,
				CaptureDir:                  "/tmp/some/path",
				MaximumCaptureFileSizeBytes: 5,
				Tags:                        []string{"a", "b", "c"},
			})
		})
	})

	t.Run("syncConfig())", func(t *testing.T) {
		t.Run("returns a sync config with defaults when called on an empty config", func(t *testing.T) {
			c := &Config{}
			test.That(t, c.syncConfig(nil, false), test.ShouldResemble, sync.Config{
				CaptureDir:                 viamCaptureDotDir,
				DeleteEveryNthWhenDiskFull: 5,
				FileLastModifiedMillis:     10000,
				MaximumNumSyncThreads:      runtime.NumCPU() / 2,
				SyncIntervalMins:           0.1,
			})
		})

		t.Run("returns a sync config with defaults when called on a config with SyncIntervalMins which is practically 0", func(t *testing.T) {
			c := &Config{SyncIntervalMins: 0.000000000000000001}
			test.That(t, c.syncConfig(nil, false), test.ShouldResemble, sync.Config{
				CaptureDir:                 viamCaptureDotDir,
				DeleteEveryNthWhenDiskFull: 5,
				FileLastModifiedMillis:     10000,
				MaximumNumSyncThreads:      runtime.NumCPU() / 2,
				SyncIntervalMins:           0.1,
			})
		})
		t.Run("returns a sync config with overridden defaults when called on a full config", func(t *testing.T) {
			s := &inject.Sensor{}
			test.That(t, fullConfig.syncConfig(s, true), test.ShouldResemble, sync.Config{
				AdditionalSyncPaths:        []string{"/tmp/a", "/tmp/b"},
				CaptureDir:                 "/tmp/some/path",
				CaptureDisabled:            true,
				DeleteEveryNthWhenDiskFull: 2,
				FileLastModifiedMillis:     50000,
				MaximumNumSyncThreads:      10,
				ScheduledSyncDisabled:      true,
				SelectiveSyncSensor:        s,
				SelectiveSyncSensorEnabled: true,
				SelectiveSyncerName:        "some name",
				SyncIntervalMins:           0.5,
				Tags:                       []string{"a", "b", "c"},
			})
		})
	})
}
