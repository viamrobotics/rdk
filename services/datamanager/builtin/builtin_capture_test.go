package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
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

func TestSwitchResource(t *testing.T) {
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

	dataBeforeSwitch, err := getSensorData(captureDir)
	test.That(t, err, test.ShouldBeNil)

	// Test that sensor data is captured from the new collector.
	waitForCaptureFilesToExceedNFiles(captureDir, len(getAllFileInfos(captureDir)), logger)
	testFilesContainSensorData(t, captureDir)

	filePaths := getAllFilePaths(captureDir)
	test.That(t, len(filePaths), test.ShouldEqual, 2)

	initialData, err := data.SensorDataFromCaptureFilePath(filePaths[0])
	test.That(t, err, test.ShouldBeNil)
	for _, d := range initialData {
		// Each resource's mocked capture method outputs a different value.
		// Assert that we see the expected data captured by the initial arm1 resource.
		test.That(
			t,
			d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()["x"].GetNumberValue(),
			test.ShouldEqual,
			float64(0),
		)
	}
	// Assert that the initial arm1 resource isn't capturing any more data.
	test.That(t, len(initialData), test.ShouldEqual, len(dataBeforeSwitch))

	newData, err := data.SensorDataFromCaptureFilePath(filePaths[1])
	test.That(t, err, test.ShouldBeNil)
	for _, d := range newData {
		// Assert that we see the expected data captured by the updated arm1 resource.
		test.That(
			t,
			d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()["x"].GetNumberValue(),
			test.ShouldEqual,
			float64(888),
		)
	}
	// Assert that the updated arm1 resource is capturing data.
	test.That(t, len(newData), test.ShouldBeGreaterThan, 0)
}

func getSensorData(dir string) ([]*v1.SensorData, error) {
	var sd []*v1.SensorData
	filePaths := getAllFilePaths(dir)
	for _, path := range filePaths {
		d, err := data.SensorDataFromCaptureFilePath(path)
		// It's possible a file was closed (and so its extension changed) in between the points where we gathered
		// file names and here. So if the file does not exist, check if the extension has just been changed.
		if errors.Is(err, os.ErrNotExist) {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + data.CompletedCaptureFileExt
			d, err = data.SensorDataFromCaptureFilePath(path)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
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
		nonEmptyFiles := 0
		for idx := range files {
			if files[idx].Size() > int64(emptyFileBytesSize) {
				// Every datamanager file has at least 90 bytes of metadata. Wait for that to be
				// observed before considering the file as "existing".
				nonEmptyFiles++
			}

			// We have N+1 files. No need to count any more.
			if nonEmptyFiles > n {
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
