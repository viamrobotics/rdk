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
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/test"
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
				fileContents := []byte("never gonna give you up")
				// overwrite thresholds
				fsThresholdToTriggerDeletion = tc.testThreshold
				captureDirToFSUsageRatio = tc.testCaptureDirRatio
				// write testing files
				for i := 0; i < 10; i++ {
					err := os.WriteFile(fmt.Sprintf("%s/file_%d", tempCaptureDir, i), fileContents, 0755)
					test.That(t, err, test.ShouldBeNil)
				}
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

// delete w/ syncer, delete w/o syncer, only .prog files, context cancellation?
// make files, mark some in progress? then call delete, then walk and see whats been deletd
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
			var syncer datasync.Manager = nil
			if tc.syncEnabled {
				s, err := datasync.NewManager("rick astley", mockClient, logger, tempCaptureDir)
				test.That(t, err, test.ShouldBeNil)
				syncer = s
				defer syncer.Close()
			}
			fileContents := []byte("never gonna let you down")
			for i := 0; i < 10; i++ {
				var filename string
				if tc.writeProgFiles {
					filename = fmt.Sprintf("%s/file_%d.prog", tempCaptureDir, i)
				} else {
					filename = fmt.Sprintf("%s/file_%d.capture", tempCaptureDir, i)
				}
				err := os.WriteFile(filename, fileContents, 0755)
				test.That(t, err, test.ShouldBeNil)
				if tc.markAllInProgress {
					ret := syncer.MarkInProgress(filename)
					test.That(t, ret, test.ShouldBeTrue)
				}
			}
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
