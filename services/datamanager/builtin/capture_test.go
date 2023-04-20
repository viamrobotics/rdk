package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/test"
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
)

func TestDataCaptureEnabled(t *testing.T) {
	// passTime repeatedly increments mc by interval until the context is canceled.
	passTime := func(ctx context.Context, mc *clk.Mock, interval time.Duration) chan struct{} {
		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-ctx.Done():
					close(done)
					return
				default:
					mc.Add(interval)
				}
			}
		}()
		return done
	}

	captureInterval := time.Millisecond * 10
	testFilesContainSensorData := func(t *testing.T, dir string) {
		t.Helper()
		var sd []*v1.SensorData
		filePaths := getAllFilePaths(dir)
		for _, path := range filePaths {
			d, err := datacapture.SensorDataFromFilePath(path)
			// It's possible a file was closed (and so its extension changed) in between the points where we gathered
			// file names and here. So if the file does not exist, check if the extension has just been changed.
			if errors.Is(err, os.ErrNotExist) {
				path = strings.TrimSuffix(path, filepath.Ext(path)) + datacapture.FileExt
				d, err = datacapture.SensorDataFromFilePath(path)
			}
			test.That(t, err, test.ShouldBeNil)
			sd = append(sd, d...)
		}

		test.That(t, len(sd), test.ShouldBeGreaterThan, 0)
		for _, d := range sd {
			test.That(t, d.GetStruct(), test.ShouldNotBeNil)
			test.That(t, d.GetMetadata(), test.ShouldNotBeNil)
		}
	}

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
			name:                          "config with data capture service disabled should capture nothing",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "config with data capture service enabled and a configured collector should capture data",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "config with data capture service implicitly enabled and a configured collector should capture data",
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
			if tc.remoteCollector {
				initConfig, deps = setupConfig(t, remoteCollectorConfigPath)
			} else if tc.initialCollectorDisableStatus {
				initConfig, deps = setupConfig(t, disabledTabularCollectorConfigPath)
			} else if tc.emptyTabular {
				initConfig, deps = setupConfig(t, enabledTabularCollectorEmptyConfigPath)
			} else {
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
				waitForCaptureFiles(initCaptureDir)
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
				waitForCaptureFiles(updatedCaptureDir)
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

func waitForCaptureFiles(captureDir string) {
	totalWait := time.Second * 2
	waitPerCheck := time.Millisecond * 10
	iterations := int(totalWait / waitPerCheck)
	files := getAllFileInfos(captureDir)
	for i := 0; i < iterations; i++ {
		if len(files) > 0 && files[0].Size() > int64(emptyFileBytesSize) {
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
		test.That(t, err, test.ShouldBeNil)
		resources[resName] = res
	}
	return resources
}
