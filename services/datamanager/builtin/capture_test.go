package builtin

import (
	"context"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"testing"
	"time"

	"go.viam.com/rdk/config"
	"go.viam.com/test"
)

var (
	// Robot config which specifies data manager service.
	enabledTabularCollectorConfigPath           = "services/datamanager/data/fake_robot_with_data_manager.json"
	disabledTabularCollectorConfigPath          = "services/datamanager/data/fake_robot_with_disabled_collector.json"
	enabledBinaryCollectorConfigPath            = "services/datamanager/data/robot_with_cam_capture.json"
	infrequentCaptureTabularCollectorConfigPath = "services/datamanager/data/fake_robot_with_infrequent_capture.json"
	remoteCollectorConfigPath                   = "services/datamanager/data/fake_robot_with_remote_and_data_manager.json"
	emptyFileBytesSize                          = 100 // size of leading metadata message
)

func TestDataCaptureEnabled(t *testing.T) {
	captureTime := time.Millisecond * 25

	// On slower machines, it's possible that capture hasn't begun after captureTime. Give up to 20x
	// as long for this to occur.
	waitForCaptureFiles := func(captureDir string) {
		files := getAllFileInfos(captureDir)
		for i := 0; i < 50; i++ {
			if len(files) > 0 && files[0].Size() > int64(emptyFileBytesSize) {
				return
			}
			time.Sleep(captureTime)
			files = getAllFileInfos(captureDir)
		}
	}

	testFilesContainSensorData := func(t *testing.T, dir string) {
		var sd []*v1.SensorData
		filePaths := getAllFilePaths(dir)
		for _, path := range filePaths {
			d, err := datacapture.SensorDataFromFilePath(path)
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
	}{
		{
			name:                          "Config with data capture service disabled should capture nothing.",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "Config with data capture service enabled and a configured collector should capture data.",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "Disabling data capture service should cause all data capture to stop.",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "Enabling data capture should cause all enabled collectors to start capturing data.",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "Enabling a collector should not trigger data capture if the service is disabled.",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "Disabling an individual collector should stop it.",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "Enabling an individual collector should start it.",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     false,
		},
		{
			name:            "Capture should work for remotes too.",
			remoteCollector: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up capture directories.
			initCaptureDir := t.TempDir()
			updatedCaptureDir := t.TempDir()

			// Set up robot config.
			var initConfig *config.Config
			if tc.remoteCollector {
				initConfig = setupConfig(t, remoteCollectorConfigPath)
			} else if tc.initialCollectorDisableStatus {
				initConfig = setupConfig(t, disabledTabularCollectorConfigPath)
			} else {
				initConfig = setupConfig(t, enabledTabularCollectorConfigPath)
			}

			// Set up service config.
			initSvcConfig, ok1, err := getServiceConfig(initConfig)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			initSvcConfig.CaptureDisabled = tc.initialServiceDisableStatus
			initSvcConfig.ScheduledSyncDisabled = true
			initSvcConfig.CaptureDir = initCaptureDir

			// Build and start data manager.
			dmsvc := newTestDataManager(t)
			defer func() {
				test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
			}()
			err = dmsvc.Update(context.Background(), initConfig)
			test.That(t, err, test.ShouldBeNil)
			time.Sleep(captureTime)
			if !tc.initialServiceDisableStatus && !tc.initialCollectorDisableStatus {
				waitForCaptureFiles(initCaptureDir)
				testFilesContainSensorData(t, initCaptureDir)
			} else {
				initialCaptureFiles := getAllFileInfos(initCaptureDir)
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}

			// Set up updated robot config.
			var updatedConfig *config.Config
			if tc.newCollectorDisableStatus {
				updatedConfig = setupConfig(t, disabledTabularCollectorConfigPath)
			} else {
				updatedConfig = setupConfig(t, enabledTabularCollectorConfigPath)
			}

			// Set up updated service config.
			updatedServiceConfig, ok, err := getServiceConfig(updatedConfig)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			updatedServiceConfig.CaptureDisabled = tc.newServiceDisableStatus
			updatedServiceConfig.ScheduledSyncDisabled = true
			updatedServiceConfig.CaptureDir = updatedCaptureDir

			// Update to new config and let it run for a bit.
			err = dmsvc.Update(context.Background(), updatedConfig)
			test.That(t, err, test.ShouldBeNil)
			oldCaptureDirFiles := getAllFileInfos(initCaptureDir)

			time.Sleep(captureTime)
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
		})
	}
}
