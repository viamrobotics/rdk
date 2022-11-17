package builtin

import (
	"context"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/test"
	"os"
	"testing"
	"time"
)

/*
*
New test setup!
First data capture only tests:
- Captures what we'd expect. Both contents and amount, to where we expect it.
- Disabling capture stops it. Re-enabling re-begins it.
*/
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
		// TODO: add test cases for when an individual collector is enabled/disabled
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

			// Set up service config.
			testCfg := setupConfig(t, configPath)
			svcConfig, ok, err := getServiceConfig(testCfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = tc.initialServiceDisableStatus
			svcConfig.ScheduledSyncDisabled = true
			svcConfig.CaptureDir = tmpDir

			// TODO: Figure out how to edit component configs such that the changes are actually reflected in the
			//       original config.
			componentConfigs, err := getComponentConfigs(testCfg)
			test.That(t, err, test.ShouldBeNil)
			for _, attr := range componentConfigs.Attributes {
				attr.Disabled = tc.initialCollectorDisableStatus
			}

			err = dmsvc.Update(context.Background(), testCfg)

			// Let run for a second, then change status.
			time.Sleep(captureTime)
			svcConfig.CaptureDisabled = tc.newServiceDisableStatus
			for _, attr := range componentConfigs.Attributes {
				attr.Disabled = tc.newCollectorDisableStatus
			}
			err = dmsvc.Update(context.Background(), testCfg)

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
				// TODO: check contents
				test.That(t, len(updatedCaptureFiles), test.ShouldBeGreaterThan, len(initialCaptureFiles))
			} else {
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, len(initialCaptureFiles))
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

func getComponentConfigs(cfg *config.Config) (*dataCaptureConfigs, error) {
	var componentDataCaptureConfigs dataCaptureConfigs
	for _, c := range cfg.Components {
		// Iterate over all component-level service configs of type data_manager.
		for _, componentSvcConfig := range c.ServiceConfig {
			if componentSvcConfig.Type == datamanager.SubtypeName {
				attrs, err := getAttrsFromServiceConfig(componentSvcConfig)
				if err != nil {
					return nil, err
				}
				for _, attrs := range attrs.Attributes {
					componentDataCaptureConfigs.Attributes = append(componentDataCaptureConfigs.Attributes, attrs)
				}
			}
		}
	}
	return &componentDataCaptureConfigs, nil
}
