package builtin

import (
	"context"
	clk "github.com/benbjohnson/clock"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datacapture"
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
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock

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
