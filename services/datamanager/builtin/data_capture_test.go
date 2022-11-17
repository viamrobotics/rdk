package builtin

import (
	"context"
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
		name                 string
		initialDisableStatus bool
		newDisableStatus     bool
	}{
		{
			"Config with data capture service disabled should capture nothing.",
			true,
			true,
		},
		{
			"Config with data capture service enabled and a configured collector should capture data.",
			false,
			false,
		},
		{
			"Disabling data capture service should cause all data capture to stop.",
			false,
			true,
		},
		{
			"Enabling data capture should cause all configured collectors to capture data.",
			true,
			false,
		},
		// TODO: add test cases for when an individual collector is enabled/disabled
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := os.TempDir()

			rpcServer, _ := buildAndStartLocalSyncServer(t)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()
			dmsvc := newTestDataManager(t, "arm1", "")
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
			testCfg := setupConfig(t, configPath)
			svcConfig, ok, err := getServiceConfig(testCfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = tc.initialDisableStatus
			svcConfig.ScheduledSyncDisabled = true
			svcConfig.CaptureDir = tmpDir
			err = dmsvc.Update(context.Background(), testCfg)

			// Let run for a second, then change status.
			time.Sleep(captureTime)
			svcConfig.CaptureDisabled = tc.newDisableStatus
			err = dmsvc.Update(context.Background(), testCfg)

			// Check if data has been captured (or not) as we'd expect.
			initialCaptureFiles := getAllFiles(tmpDir)
			if !tc.initialDisableStatus {
				// TODO: check contents
				test.That(t, len(initialCaptureFiles), test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}

			// Let run for a second.
			time.Sleep(captureTime)
			// Check if data has been captured (or not) as we'd expect.
			updatedCaptureFiles := getAllFiles(tmpDir)
			if !tc.newDisableStatus {
				// TODO: check contents
				test.That(t, len(updatedCaptureFiles), test.ShouldBeGreaterThan, len(initialCaptureFiles))
			} else {
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, len(initialCaptureFiles))
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
	}
}
