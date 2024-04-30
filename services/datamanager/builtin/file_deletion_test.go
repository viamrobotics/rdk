package builtin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datasync"
)

func TestFileDeletionUsageCheck(t *testing.T) {
	tests := []struct {
		name                 string
		expectedFsCheckValue bool
		testThreshold        float64
		testCaptureDirRatio  float64
		captureDirExists     bool
	}{
		{
			name:                 "if not at file system capactiy threshold, we should return false from deletion check",
			expectedFsCheckValue: false,
			testThreshold:        .99,
			testCaptureDirRatio:  .99,
			captureDirExists:     true,
		},
		{
			name:                 "if at file system capactiy threshold, we return true from deletion check",
			expectedFsCheckValue: true,
			testThreshold:        math.SmallestNonzeroFloat64,
			testCaptureDirRatio:  math.SmallestNonzeroFloat64,
			captureDirExists:     true,
		},
		{
			name: "if at file system capactiy threshold but not capture dir threshold," +
				"we return false from deletion check",
			expectedFsCheckValue: false,
			testThreshold:        math.SmallestNonzeroFloat64,
			testCaptureDirRatio:  1.0,
			captureDirExists:     true,
		},
		{
			name:                 "if capture dir does not exist, we should return false from deletion check",
			expectedFsCheckValue: false,
			testThreshold:        .95,
			testCaptureDirRatio:  .5,
			captureDirExists:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tempCaptureDir string
			if tc.captureDirExists {
				tempCaptureDir = t.TempDir()
				// overwrite thresholds
				fsThresholdToTriggerDeletion = tc.testThreshold
				captureDirToFSUsageRatio = tc.testCaptureDirRatio
				// write testing files
				writeFiles(t, tempCaptureDir, false)
			} else {
				// make a random dir name to make sure it doesn't exist
				randomName := make([]byte, 4)
				_, err := rand.Read(randomName)
				test.That(t, err, test.ShouldBeNil)
				tempCaptureDir = hex.EncodeToString(randomName)
			}

			logger := logging.NewTestLogger(t)
			willDelete, err := shouldDeleteBasedOnDiskUsage(context.Background(), tempCaptureDir, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, willDelete, test.ShouldEqual, tc.expectedFsCheckValue)
		})
	}
}

func TestFileDeletion(t *testing.T) {
	tests := []struct {
		name                 string
		syncEnabled          bool
		markAllInProgress    bool
		writeProgFiles       bool
		shouldCancelContext  bool
		expectedDeletedCount int
	}{
		{
			name:                 "deletion with sync disabled should delete every 4th file",
			expectedDeletedCount: 3,
		},
		{
			name:                 "deletion with sync enabled and all files marked as in progress should not delete any files",
			syncEnabled:          true,
			markAllInProgress:    true,
			expectedDeletedCount: 0,
		},
		{
			name:                 "deletion with sync disabled and files still being written to should not delete any files",
			writeProgFiles:       true,
			expectedDeletedCount: 0,
		},
		{
			name:                 "deletion with sync disabled and files still being written to should not delete any files",
			writeProgFiles:       true,
			expectedDeletedCount: 0,
		},
		{
			name:                "deletion with a cancelled context should return an error",
			shouldCancelContext: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempCaptureDir := t.TempDir()
			logger := logging.NewTestLogger(t)
			mockClient := mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}

			var syncer datasync.Manager
			if tc.syncEnabled {
				s, err := datasync.NewManager("rick astley", mockClient, logger, tempCaptureDir)
				test.That(t, err, test.ShouldBeNil)
				syncer = s
				defer syncer.Close()
			}

			writeFilesAndMarkInProgress(t, tempCaptureDir, tc.writeProgFiles, syncer)

			ctx, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()
			if tc.shouldCancelContext {
				cancelFunc()
			}
			deletedFileCount, err := deleteFiles(ctx, syncer, tempCaptureDir, logger)
			if tc.shouldCancelContext {
				test.That(t, err, test.ShouldBeError, context.Canceled)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, deletedFileCount, test.ShouldEqual, tc.expectedDeletedCount)
			}
		})
	}
}

func writeFiles(t *testing.T, dir string, writeFilesAsUnfinished bool) []string {
	t.Helper()
	filenames := []string{}
	fileContents := []byte("never gonna let you down")
	for i := 0; i < 10; i++ {
		var filename string
		if writeFilesAsUnfinished {
			filename = fmt.Sprintf("%s/file_%d.prog", dir, i)
		} else {
			filename = fmt.Sprintf("%s/file_%d.capture", dir, i)
		}
		err := os.WriteFile(filename, fileContents, 0o755)
		test.That(t, err, test.ShouldBeNil)
		filenames = append(filenames, filename)
	}
	return filenames
}

func writeFilesAndMarkInProgress(t *testing.T, dir string, writeFilesAsUnfinished bool, syncer datasync.Manager) {
	t.Helper()
	files := writeFiles(t, dir, writeFilesAsUnfinished)
	for _, file := range files {
		if syncer != nil {
			ret := syncer.MarkInProgress(file)
			test.That(t, ret, test.ShouldBeTrue)
		}
	}
}
