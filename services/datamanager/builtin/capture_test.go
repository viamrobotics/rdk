package builtin

import (
	"context"
	"go.viam.com/rdk/config"
	"go.viam.com/test"
	"os"
	"testing"
	"time"
)

var (
	// Robot config which specifies data manager service.
	enabledCollectorConfigPath  = "services/datamanager/data/fake_robot_with_data_manager.json"
	disabledCollectorConfigPath = "services/datamanager/data/fake_robot_with_disabled_collector.json"
)

func TestDataCaptureEnabled(t *testing.T) {
	captureTime := time.Millisecond * 25

	tests := []struct {
		name                          string
		initialServiceDisableStatus   bool
		newServiceDisableStatus       bool
		initialCollectorDisableStatus bool
		newCollectorDisableStatus     bool
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			tmpDir, err := os.MkdirTemp("", "")
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				err := os.RemoveAll(tmpDir)
				test.That(t, err, test.ShouldBeNil)
			}()
			rpcServer, _ := buildAndStartLocalSyncServer(t)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t, "arm1", "")
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))

			var initialConfig *config.Config
			if tc.initialCollectorDisableStatus {
				initialConfig = setupConfig(t, disabledCollectorConfigPath)
			} else {
				initialConfig = setupConfig(t, enabledCollectorConfigPath)
			}

			// Set up service config.
			originalSvcConfig, ok1, err := getServiceConfig(initialConfig)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			originalSvcConfig.CaptureDisabled = tc.initialServiceDisableStatus
			originalSvcConfig.ScheduledSyncDisabled = true
			originalSvcConfig.CaptureDir = tmpDir

			// TODO: Figure out how to edit component configs such that the changes are actually reflected in the
			//       original config.

			err = dmsvc.Update(context.Background(), initialConfig)

			// Let run for a second, then change status.
			time.Sleep(captureTime)

			// Set up service config.
			var updatedConfig *config.Config
			if tc.newCollectorDisableStatus {
				updatedConfig = setupConfig(t, disabledCollectorConfigPath)
			} else {
				updatedConfig = setupConfig(t, enabledCollectorConfigPath)
			}

			updatedServiceConfig, ok, err := getServiceConfig(updatedConfig)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			updatedServiceConfig.CaptureDisabled = tc.newServiceDisableStatus
			updatedServiceConfig.ScheduledSyncDisabled = true
			updatedServiceConfig.CaptureDir = tmpDir
			err = dmsvc.Update(context.Background(), updatedConfig)

			// Check if data has been captured (or not) as we'd expect.
			initialCaptureFiles := getAllFiles(tmpDir)
			if !tc.initialServiceDisableStatus && !tc.initialCollectorDisableStatus {
				// TODO: check contents
				test.That(t, len(initialCaptureFiles), test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}

			// Let run for a second.
			time.Sleep(captureTime)
			// Check if data has been captured (or not) as we'd expect.
			updatedCaptureFiles := getAllFiles(tmpDir)
			if !tc.newServiceDisableStatus && !tc.newCollectorDisableStatus {
				//TODO: check contents
				test.That(t, len(updatedCaptureFiles), test.ShouldBeGreaterThan, len(initialCaptureFiles))
			} else {
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, len(initialCaptureFiles))
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
	}
}
