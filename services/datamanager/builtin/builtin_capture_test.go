package builtin

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestDataCaptureEnabled(t *testing.T) {
	tests := []struct {
		name                          string
		initialServiceDisableStatus   bool
		newServiceDisableStatus       bool
		initialCollectorDisableStatus bool
		newCollectorDisableStatus     bool
		remoteCollector               bool
	}{
		{
			name:                          "data capture service disabled, should capture nothing",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "data capture service enabled and a configured collector, should capture data",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},

		{
			name:                          "disabling data capture service should cause all data capture to stop",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "enabling data capture should cause all enabled collectors to start capturing data",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "enabling a collector should not trigger data capture if the service is disabled",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "disabling an individual collector should stop it",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "enabling an individual collector should start it",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     false,
		},
		{
			name:            "capture should work for remotes too",
			remoteCollector: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := logging.NewTestLogger(t)
			// Set up capture directories.
			initCaptureDir := t.TempDir()
			updatedCaptureDir := t.TempDir()

			// Set up robot config.
			var config resource.Config
			var deps resource.Dependencies
			var r *inject.Robot

			switch {
			case tc.remoteCollector:
				injectedRemoteArm := &inject.Arm{}
				injectedRemoteArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
					return spatialmath.NewZeroPose(), nil
				}
				r = setupRobot(nil, map[resource.Name]resource.Resource{
					arm.Named("arm1"): &inject.Arm{
						EndPositionFunc: func(
							ctx context.Context,
							extra map[string]interface{},
						) (spatialmath.Pose, error) {
							return spatialmath.NewZeroPose(), nil
						},
					},
					arm.Named("remote1:remoteArm"): injectedRemoteArm,
				})
				config, deps = setupConfig(t, r, remoteCollectorConfigPath)
			case tc.initialCollectorDisableStatus:
				r = setupRobot(nil, map[resource.Name]resource.Resource{
					arm.Named("arm1"): &inject.Arm{
						EndPositionFunc: func(
							ctx context.Context,
							extra map[string]interface{},
						) (spatialmath.Pose, error) {
							return spatialmath.NewZeroPose(), nil
						},
					},
				})
				config, deps = setupConfig(t, r, disabledTabularCollectorConfigPath)
			default:
				r = setupRobot(nil, map[resource.Name]resource.Resource{
					arm.Named("arm1"): &inject.Arm{
						EndPositionFunc: func(
							ctx context.Context,
							extra map[string]interface{},
						) (spatialmath.Pose, error) {
							return spatialmath.NewZeroPose(), nil
						},
					},
				})
				config, deps = setupConfig(t, r, enabledTabularCollectorConfigPath)
			}
			c := config.ConvertedAttributes.(*Config)
			// further set up service config.
			c.CaptureDir = initCaptureDir
			c.CaptureDisabled = tc.initialServiceDisableStatus
			c.ScheduledSyncDisabled = true
			// MaximumCaptureFileSizeBytes is set to 1 so that each reading becomes its own capture file
			// and we can confidently read the capture file without it's contents being modified by the collector
			c.MaximumCaptureFileSizeBytes = 1

			// Build and start data manager.
			b, err := New(context.Background(), deps, config, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
			test.That(t, err, test.ShouldBeNil)
			defer func() { test.That(t, b.Close(context.Background()), test.ShouldBeNil) }()

			logger.Warnf("calling waitForCaptureFilesToExceedNFiles with dir: %s", initCaptureDir)
			if !tc.initialServiceDisableStatus && !tc.initialCollectorDisableStatus {
				waitForCaptureFilesToExceedNFiles(initCaptureDir, 0, logger)
				testFilesContainSensorData(t, initCaptureDir)
			} else {
				initialCaptureFiles := getAllFileInfos(initCaptureDir)
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}

			// Set up updated robot config.
			var updatedConfig resource.Config
			if tc.newCollectorDisableStatus {
				updatedConfig, deps = setupConfig(t, r, disabledTabularCollectorConfigPath)
			} else {
				updatedConfig, deps = setupConfig(t, r, enabledTabularCollectorConfigPath)
			}
			c2 := updatedConfig.ConvertedAttributes.(*Config)

			// further set up updated service config.
			c2.CaptureDisabled = tc.newServiceDisableStatus
			c2.ScheduledSyncDisabled = true
			c2.CaptureDir = updatedCaptureDir
			// MaximumCaptureFileSizeBytes is set to 1 so that each reading becomes its own capture file
			// and we can confidently read the capture file without it's contents being modified by the collector
			c2.MaximumCaptureFileSizeBytes = 1

			// Update to new config and let it run for a bit.
			err = b.Reconfigure(context.Background(), deps, updatedConfig)
			test.That(t, err, test.ShouldBeNil)
			oldCaptureDirFiles := getAllFileInfos(initCaptureDir)

			if !tc.newServiceDisableStatus && !tc.newCollectorDisableStatus {
				waitForCaptureFilesToExceedNFiles(updatedCaptureDir, 0, logger)
				testFilesContainSensorData(t, updatedCaptureDir)
			} else {
				updatedCaptureFiles := getAllFileInfos(updatedCaptureDir)
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, 0)
				oldCaptureDirFilesAfterWait := getAllFileInfos(initCaptureDir)
				test.That(t, len(oldCaptureDirFilesAfterWait), test.ShouldEqual, len(oldCaptureDirFiles))
				for i := range oldCaptureDirFiles {
					test.That(t, oldCaptureDirFiles[i].Size(), test.ShouldEqual, oldCaptureDirFilesAfterWait[i].Size())
				}
			}
		})
	}
}

func TestReconfigureResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	captureDir := t.TempDir()

	// Set up robot config.
	r := setupRobot(nil, map[resource.Name]resource.Resource{
		arm.Named("arm1"): &inject.Arm{
			EndPositionFunc: func(
				ctx context.Context,
				extra map[string]interface{},
			) (spatialmath.Pose, error) {
				return spatialmath.NewZeroPose(), nil
			},
		},
	})
	config, deps := setupConfig(t, r, enabledTabularCollectorConfigPath)
	c := config.ConvertedAttributes.(*Config)
	c.CaptureDisabled = false
	c.ScheduledSyncDisabled = true
	c.CaptureDir = captureDir
	// MaximumCaptureFileSizeBytes is set to 1 so that each reading becomes its own capture file
	// and we can confidently read the capture file without it's contents being modified by the collector
	c.MaximumCaptureFileSizeBytes = 1

	// Build and start data manager.
	b, err := New(context.Background(), deps, config, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, b.Close(context.Background()), test.ShouldBeNil)
	}()

	waitForCaptureFilesToExceedNFiles(captureDir, 0, logger)
	testFilesContainSensorData(t, captureDir)

	// Change the resource named arm1 to show that the data manager recognizes that the collector has changed with no other config changes.
	r2 := setupRobot(nil, map[resource.Name]resource.Resource{
		arm.Named("arm1"): &inject.Arm{
			EndPositionFunc: func(
				ctx context.Context,
				extra map[string]interface{},
			) (spatialmath.Pose, error) {
				return spatialmath.NewPoseFromPoint(r3.Vector{X: 888, Y: 888, Z: 888}), nil
			},
		},
	})
	_, deps2 := setupConfig(t, r2, enabledTabularCollectorConfigPath)

	err = b.Reconfigure(context.Background(), deps2, config)
	test.That(t, err, test.ShouldBeNil)

	// wait for all the files on disk to
	waitForCaptureFilesToExceedNFiles(captureDir, len(getAllFileInfos(captureDir)), logger)
	testFilesContainSensorData(t, captureDir)

	// Test that sensor data is captured from the new collector.
	var (
		captureDataHasZeroReadings    bool
		captureDataHasNonZeroReadings bool
	)

	for _, fp := range getAllFilePaths(captureDir) {
		// ignore in progress files
		if filepath.Ext(fp) == data.InProgressCaptureFileExt {
			continue
		}
		initialData, err := data.SensorDataFromCaptureFilePath(fp)
		test.That(t, err, test.ShouldBeNil)
		for _, d := range initialData {
			// Each resource's mocked capture method outputs a different value.
			// Assert that we see the expected data captured by the initial arm1 resource.
			pose := d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()
			if pose["x"].GetNumberValue() == 0 && pose["y"].GetNumberValue() == 0 && pose["z"].GetNumberValue() == 0 {
				captureDataHasZeroReadings = true
			}

			if pose["x"].GetNumberValue() == 888 && pose["y"].GetNumberValue() == 888 && pose["z"].GetNumberValue() == 888 {
				captureDataHasNonZeroReadings = true
			}
		}
	}

	// Assert that both the sensor data from the first instance of `arm1` was captured as well as data from the second instance
	test.That(t, captureDataHasZeroReadings, test.ShouldBeTrue)
	test.That(t, captureDataHasNonZeroReadings, test.ShouldBeTrue)
}

func getSensorData(dir string) ([]*v1.SensorData, error) {
	var sd []*v1.SensorData
	filePaths := getAllFilePaths(dir)
	for _, path := range filePaths {
		if filepath.Ext(path) == data.InProgressCaptureFileExt {
			continue
		}
		d, err := data.SensorDataFromCaptureFilePath(path)
		if err != nil {
			return nil, err
		}
		sd = append(sd, d...)
	}
	return sd, nil
}

// testFilesContainSensorData verifies that the files in `dir` contain sensor data.
func testFilesContainSensorData(t *testing.T, dir string) {
	t.Helper()

	sd, err := getSensorData(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(sd), test.ShouldBeGreaterThan, 0)
	for _, d := range sd {
		test.That(t, d.GetStruct(), test.ShouldNotBeNil)
		test.That(t, d.GetMetadata(), test.ShouldNotBeNil)
	}
}

// waitForCaptureFilesToExceedNFiles returns once `captureDir` contains more than `n` files of at
// least `emptyFileBytesSize` bytes. This is not suitable for waiting for file deletion to happen.
func waitForCaptureFilesToExceedNFiles(captureDir string, n int, logger logging.Logger) {
	var diagnostics sync.Once
	start := time.Now()
	for {
		files := getAllFileInfos(captureDir)
		captureFiles := 0
		for idx := range files {
			if files[idx].Size() > int64(emptyFileBytesSize) && filepath.Ext(files[idx].Name()) == data.CompletedCaptureFileExt {
				// Every datamanager file has at least 90 bytes of metadata. Wait for that to be
				// observed before considering the file as "existing".
				captureFiles++
			}

			// We have N+1 files. No need to count any more.
			if captureFiles > n {
				return
			}
		}

		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 10*time.Second {
			diagnostics.Do(func() {
				logger.Infow("waitForCaptureFilesToEqualNFiles diagnostics after 10 seconds of waiting", "numFiles", len(files), "expectedFiles", n)
				for idx, file := range files {
					logger.Infow("File information", "idx", idx, "dir", captureDir, "name", file.Name(), "size", file.Size())
				}
			})
		}
	}
}
