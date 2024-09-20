package sync

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/testutils/inject"
)

func TestConfig(t *testing.T) {
	sensorA := &inject.Sensor{}
	sensorB := &inject.Sensor{}
	t.Run("Equal()", func(t *testing.T) {
		type testCase struct {
			name  string
			a     Config
			b     Config
			equal bool
		}
		tcs := []testCase{
			{
				name:  "empty configs are equal",
				a:     Config{},
				b:     Config{},
				equal: true,
			},
			{
				name: "full configs that are equal are equal",
				a: Config{
					AdditionalSyncPaths:        []string{"a", "b"},
					CaptureDir:                 "c",
					CaptureDisabled:            true,
					DeleteEveryNthWhenDiskFull: 5,
					FileLastModifiedMillis:     100,
					MaximumNumSyncThreads:      100,
					ScheduledSyncDisabled:      true,
					SelectiveSyncerName:        "tom",
					SyncIntervalMins:           1.1,
					Tags:                       []string{"c", "d"},
					SelectiveSyncSensorEnabled: true,
					SelectiveSyncSensor:        sensorA,
				},
				b: Config{
					AdditionalSyncPaths:        []string{"a", "b"},
					CaptureDir:                 "c",
					CaptureDisabled:            true,
					DeleteEveryNthWhenDiskFull: 5,
					FileLastModifiedMillis:     100,
					MaximumNumSyncThreads:      100,
					ScheduledSyncDisabled:      true,
					SelectiveSyncerName:        "tom",
					SyncIntervalMins:           1.1,
					Tags:                       []string{"c", "d"},
					SelectiveSyncSensorEnabled: true,
					SelectiveSyncSensor:        sensorA,
				},
				equal: true,
			},
			{
				name: "different AdditionalSyncPaths are not equal",
				a: Config{
					AdditionalSyncPaths: []string{"b"},
				},
				b: Config{
					AdditionalSyncPaths: []string{"a"},
				},
				equal: false,
			},
			{
				name: "different CaptureDir are not equal",
				a: Config{
					CaptureDir: "c",
				},
				b: Config{
					CaptureDir: "d",
				},
				equal: false,
			},
			{
				name: "different CaptureDisabled are not equal",
				a: Config{
					CaptureDisabled: true,
				},
				b: Config{
					CaptureDisabled: false,
				},
				equal: false,
			},
			{
				name: "different DeleteEveryNthWhenDiskFull are not equal",
				a: Config{
					DeleteEveryNthWhenDiskFull: 5,
				},
				b: Config{
					DeleteEveryNthWhenDiskFull: 4,
				},
				equal: false,
			},
			{
				name: "different FileLastModifiedMillis are not equal",
				a: Config{
					FileLastModifiedMillis: 5,
				},
				b: Config{
					FileLastModifiedMillis: 4,
				},
				equal: false,
			},
			{
				name: "different MaximumNumSyncThreads are not equal",
				a: Config{
					MaximumNumSyncThreads: 5,
				},
				b: Config{
					MaximumNumSyncThreads: 4,
				},
				equal: false,
			},
			{
				name: "different ScheduledSyncDisabled are not equal",
				a: Config{
					ScheduledSyncDisabled: true,
				},
				b: Config{
					ScheduledSyncDisabled: false,
				},
				equal: false,
			},
			{
				name: "different SelectiveSyncerName are not equal",
				a: Config{
					SelectiveSyncerName: "tom",
				},
				b: Config{
					SelectiveSyncerName: "jim",
				},
				equal: false,
			},
			{
				name: "different SyncIntervalMins are not equal",
				a: Config{
					SyncIntervalMins: 1.0,
				},
				b: Config{
					SyncIntervalMins: 1.1,
				},
				equal: false,
			},
			{
				name: "different Tags are not equal",
				a: Config{
					Tags: []string{"a"},
				},
				b: Config{
					Tags: []string{"b"},
				},
				equal: false,
			},
			{
				name: "different SelectiveSyncSensorEnabled are not equal",
				a: Config{
					SelectiveSyncSensorEnabled: true,
				},
				b: Config{
					SelectiveSyncSensorEnabled: false,
				},
				equal: false,
			},
			{
				name: "different SelectiveSyncSensor are not equal",
				a: Config{
					SelectiveSyncSensor: sensorA,
				},
				b: Config{
					SelectiveSyncSensor: sensorB,
				},
				equal: false,
			},
		}

		for _, tc := range tcs {
			t.Run(tc.name, func(t *testing.T) {
				test.That(t, tc.a.Equal(tc.b), test.ShouldEqual, tc.equal)
				test.That(t, tc.b.Equal(tc.a), test.ShouldEqual, tc.equal)
			})
		}
	})

	t.Run("schedulerEnabled()", func(t *testing.T) {
		t.Run("true by default", func(t *testing.T) {
			test.That(t, Config{}.schedulerEnabled(), test.ShouldBeTrue)
		})

		t.Run("false if ScheduledSyncDisabled", func(t *testing.T) {
			test.That(t, Config{ScheduledSyncDisabled: true}.schedulerEnabled(), test.ShouldBeFalse)
			test.That(t, Config{ScheduledSyncDisabled: true, SyncIntervalMins: 1.0}.schedulerEnabled(), test.ShouldBeFalse)
		})

		t.Run("false if SelectiveSyncSensorEnabled is true and SelectiveSyncSensor is nil", func(t *testing.T) {
			test.That(t, Config{SelectiveSyncSensorEnabled: true}.schedulerEnabled(), test.ShouldBeFalse)
			test.That(t, Config{SelectiveSyncSensorEnabled: true, SyncIntervalMins: 1.0}.schedulerEnabled(), test.ShouldBeFalse)
		})

		t.Run("true otherwise", func(t *testing.T) {
			test.That(t, Config{SyncIntervalMins: 1.0}.schedulerEnabled(), test.ShouldBeTrue)
			test.That(t, Config{
				SyncIntervalMins:           1.0,
				SelectiveSyncSensorEnabled: true,
				SelectiveSyncSensor:        &inject.Sensor{},
			}.schedulerEnabled(), test.ShouldBeTrue)
		})
	})

	t.Run("syncPaths()", func(t *testing.T) {
		captureDir := "/some/capture/dir"
		empty := Config{CaptureDir: captureDir}
		full := Config{CaptureDir: captureDir, AdditionalSyncPaths: []string{"/some/other", "/paths"}}
		test.That(t, empty.SyncPaths(), test.ShouldResemble, []string{captureDir})
		test.That(t, full.SyncPaths(), test.ShouldResemble, []string{captureDir, "/some/other", "/paths"})
	})
}
