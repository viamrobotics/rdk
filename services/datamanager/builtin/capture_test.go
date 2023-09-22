package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var (
	// Robot config which specifies data manager service.
	enabledTabularCollectorConfigPath           = "services/datamanager/data/fake_robot_with_data_manager.json"
	enabledTabularCollectorEmptyConfigPath      = "services/datamanager/data/fake_robot_with_data_manager_empty.json"
	disabledTabularCollectorConfigPath          = "services/datamanager/data/fake_robot_with_disabled_collector.json"
	enabledBinaryCollectorConfigPath            = "services/datamanager/data/robot_with_cam_capture.json"
	infrequentCaptureTabularCollectorConfigPath = "services/datamanager/data/fake_robot_with_infrequent_capture.json"
	remoteCollectorConfigPath                   = "services/datamanager/data/fake_robot_with_remote_and_data_manager.json"
	emptyFileBytesSize                          = 100 // size of leading metadata message
	captureInterval                             = time.Millisecond * 10
)

func TestDataCaptureEnabled(t *testing.T) {
	tests := []struct {
		name                          string
		initialServiceDisableStatus   bool
		newServiceDisableStatus       bool
		initialCollectorDisableStatus bool
		newCollectorDisableStatus     bool
		remoteCollector               bool
		emptyTabular                  bool
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
			name:                          "data capture service implicitly enabled and a configured collector, should capture data",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
			emptyTabular:                  true,
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
			// Set up capture directories.
			initCaptureDir := t.TempDir()
			updatedCaptureDir := t.TempDir()
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock

			// Set up robot config.
			var initConfig *Config
			var deps []string
			switch {
			case tc.remoteCollector:
				initConfig, deps = setupConfig(t, remoteCollectorConfigPath)
			case tc.initialCollectorDisableStatus:
				initConfig, deps = setupConfig(t, disabledTabularCollectorConfigPath)
			case tc.emptyTabular:
				initConfig, deps = setupConfig(t, enabledTabularCollectorEmptyConfigPath)
			default:
				initConfig, deps = setupConfig(t, enabledTabularCollectorConfigPath)
			}

			// further set up service config.
			initConfig.CaptureDisabled = tc.initialServiceDisableStatus
			initConfig.ScheduledSyncDisabled = true
			initConfig.CaptureDir = initCaptureDir

			// Build and start data manager.
			dmsvc, r := newTestDataManager(t)
			defer func() {
				test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
			}()

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: initConfig,
			})
			test.That(t, err, test.ShouldBeNil)
			passTimeCtx1, cancelPassTime1 := context.WithCancel(context.Background())
			donePassingTime1 := passTime(passTimeCtx1, mockClock, captureInterval)

			if !tc.initialServiceDisableStatus && !tc.initialCollectorDisableStatus {
				waitForCaptureFilesToExceedNFiles(initCaptureDir, 0)
				testFilesContainSensorData(t, initCaptureDir)
			} else {
				initialCaptureFiles := getAllFileInfos(initCaptureDir)
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}
			cancelPassTime1()
			<-donePassingTime1

			// Set up updated robot config.
			var updatedConfig *Config
			if tc.newCollectorDisableStatus {
				updatedConfig, deps = setupConfig(t, disabledTabularCollectorConfigPath)
			} else {
				updatedConfig, deps = setupConfig(t, enabledTabularCollectorConfigPath)
			}

			// further set up updated service config.
			updatedConfig.CaptureDisabled = tc.newServiceDisableStatus
			updatedConfig.ScheduledSyncDisabled = true
			updatedConfig.CaptureDir = updatedCaptureDir

			// Update to new config and let it run for a bit.
			resources = resourcesFromDeps(t, r, deps)
			err = dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: updatedConfig,
			})
			test.That(t, err, test.ShouldBeNil)
			oldCaptureDirFiles := getAllFileInfos(initCaptureDir)
			passTimeCtx2, cancelPassTime2 := context.WithCancel(context.Background())
			donePassingTime2 := passTime(passTimeCtx2, mockClock, captureInterval)

			if !tc.newServiceDisableStatus && !tc.newCollectorDisableStatus {
				waitForCaptureFilesToExceedNFiles(updatedCaptureDir, 0)
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
			cancelPassTime2()
			<-donePassingTime2
		})
	}
}

func TestSwitchResource(t *testing.T) {
	captureDir := t.TempDir()
	mockClock := clk.NewMock()
	// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
	clock = mockClock

	// Set up robot config.
	config, deps := setupConfig(t, enabledTabularCollectorConfigPath)
	config.CaptureDisabled = false
	config.ScheduledSyncDisabled = true
	config.CaptureDir = captureDir

	// Build and start data manager.
	dmsvc, r := newTestDataManager(t)
	defer func() {
		test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
	}()

	resources := resourcesFromDeps(t, r, deps)
	err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
		ConvertedAttributes: config,
	})
	test.That(t, err, test.ShouldBeNil)
	passTimeCtx1, cancelPassTime1 := context.WithCancel(context.Background())
	donePassingTime1 := passTime(passTimeCtx1, mockClock, captureInterval)

	waitForCaptureFilesToExceedNFiles(captureDir, 0)
	testFilesContainSensorData(t, captureDir)

	cancelPassTime1()
	<-donePassingTime1

	// Change the resource named arm1 to show that the data manager recognizes that the collector has changed with no other config changes.
	for resource := range resources {
		if resource.Name == "arm1" {
			newResource := inject.NewArm(resource.Name)
			newResource.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
				// Return a different value from the initial arm1 resource.
				return spatialmath.NewPoseFromPoint(r3.Vector{X: 888, Y: 888, Z: 888}), nil
			}
			resources[resource] = newResource
		}
	}

	err = dmsvc.Reconfigure(context.Background(), resources, resource.Config{
		ConvertedAttributes: config,
	})
	test.That(t, err, test.ShouldBeNil)

	dataBeforeSwitch, err := getSensorData(captureDir)
	test.That(t, err, test.ShouldBeNil)

	passTimeCtx2, cancelPassTime2 := context.WithCancel(context.Background())
	donePassingTime2 := passTime(passTimeCtx2, mockClock, captureInterval)

	// Test that sensor data is captured from the new collector.
	waitForCaptureFilesToExceedNFiles(captureDir, len(getAllFileInfos(captureDir)))
	testFilesContainSensorData(t, captureDir)

	filePaths := getAllFilePaths(captureDir)
	test.That(t, len(filePaths), test.ShouldEqual, 2)

	initialData, err := datacapture.SensorDataFromFilePath(filePaths[0])
	test.That(t, err, test.ShouldBeNil)
	for _, d := range initialData {
		// Each resource's mocked capture method outputs a different value.
		// Assert that we see the expected data captured by the initial arm1 resource.
		b := d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()["o_z"].GetNumberValue()
		test.That(
			t,
			d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()["o_z"].GetNumberValue(), test.ShouldEqual,
			float64(1),
		)
		fmt.Println(b)
	}
	// Assert that the initial arm1 resource isn't capturing any more data.
	test.That(t, len(initialData), test.ShouldEqual, len(dataBeforeSwitch))

	newData, err := datacapture.SensorDataFromFilePath(filePaths[1])
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

	cancelPassTime2()
	<-donePassingTime2
}

// passTime repeatedly increments mc by interval until the context is canceled.
func passTime(ctx context.Context, mc *clk.Mock, interval time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			default:
				time.Sleep(10 * time.Millisecond)
				mc.Add(interval)
			}
		}
	}()
	return done
}

func getSensorData(dir string) ([]*v1.SensorData, error) {
	var sd []*v1.SensorData
	filePaths := getAllFilePaths(dir)
	for _, path := range filePaths {
		d, err := datacapture.SensorDataFromFilePath(path)
		// It's possible a file was closed (and so its extension changed) in between the points where we gathered
		// file names and here. So if the file does not exist, check if the extension has just been changed.
		if errors.Is(err, os.ErrNotExist) {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + datacapture.FileExt
			d, err = datacapture.SensorDataFromFilePath(path)
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

// waitForCaptureFilesToExceedNFiles returns once `captureDir` contains more than `n` files.
func waitForCaptureFilesToExceedNFiles(captureDir string, n int) {
	totalWait := time.Second * 2
	waitPerCheck := time.Millisecond * 10
	iterations := int(totalWait / waitPerCheck)
	files := getAllFileInfos(captureDir)
	for i := 0; i < iterations; i++ {
		if len(files) > n && files[n].Size() > int64(emptyFileBytesSize) {
			return
		}
		time.Sleep(waitPerCheck)
		files = getAllFileInfos(captureDir)
	}
}

func resourcesFromDeps(t *testing.T, r robot.Robot, deps []string) resource.Dependencies {
	t.Helper()
	resources := resource.Dependencies{}
	for _, dep := range deps {
		resName, err := resource.NewFromString(dep)
		test.That(t, err, test.ShouldBeNil)
		res, err := r.ResourceByName(resName)
		if err == nil {
			// some resources are weakly linked
			resources[resName] = res
		}
	}
	return resources
}
