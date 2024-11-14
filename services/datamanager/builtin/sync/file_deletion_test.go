package sync

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

func TestFileDeletion(t *testing.T) {
	tests := []struct {
		name                    string
		shouldCancelContext     bool
		expectedDeleteFilenames []string
		fileList                []string
		syncerInProgressFiles   []string
	}{
		{
			name: "if sync disabled, file deleter should delete every 5th file",
			fileList: []string{
				"0shouldDelete.capture", "1.capture", "2.capture", "3.capture",
				"4.capture", "5shouldDelete.capture",
			},
			expectedDeleteFilenames: []string{"0shouldDelete.capture", "5shouldDelete.capture"},
		},
		{
			name:                    "if and all files marked as in progress, file deleter should not delete any files",
			fileList:                []string{"0.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5.capture"},
			syncerInProgressFiles:   []string{"0.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5.capture"},
			expectedDeleteFilenames: []string{},
		},
		{
			name:                    "if some files marked as inprogress, file deleter should delete less files",
			fileList:                []string{"0.capture", "1.capture", "2shouldDelete.capture", "3.capture", "4.capture", "5.capture"},
			syncerInProgressFiles:   []string{"0.capture", "1.capture"},
			expectedDeleteFilenames: []string{"2shouldDelete.capture"},
		},
		{
			name:                    "if files are still being written to, file deleter should not delete any files",
			fileList:                []string{"0.prog", "1.prog", "2.prog", "3.prog", "4.prog", "5.prog"},
			expectedDeleteFilenames: []string{},
		},
		{
			name:                    "file deleter should not delete non datacapture files",
			fileList:                []string{"0.fe", "1.fi", "2.fo", "3.fum", "4.foo", "5.capture"},
			expectedDeleteFilenames: []string{"5.capture"},
		},
		{
			name:                "if cancelled context is cancelled, file deleter should return an error",
			shouldCancelContext: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempCaptureDir := t.TempDir()
			logger := logging.NewTestLogger(t)
			ft := newFileTracker()

			filepaths := writeFiles(t, tempCaptureDir, tc.fileList)
			for _, file := range tc.syncerInProgressFiles {
				ft.markInProgress(filepaths[file])
			}

			ctx, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()
			if tc.shouldCancelContext {
				cancelFunc()
			}
			deletedFileCount, err := deleteFiles(ctx, ft, 5, tempCaptureDir, logger)
			if tc.shouldCancelContext {
				test.That(t, err, test.ShouldBeError, context.Canceled)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, deletedFileCount, test.ShouldEqual, len(tc.expectedDeleteFilenames))
				// get list of all files still in capture dir after deletion
				files := getFileNames(t, tempCaptureDir)
				for _, deletedFile := range tc.expectedDeleteFilenames {
					test.That(t, files, test.ShouldNotContain, deletedFile)
				}
			}
		})
	}
}

func TestFileDeletionUsageCheck(t *testing.T) {
	tests := []struct {
		name              string
		deletionExpected  bool
		triggerThreshold  float64
		captureUsageRatio float64
		captureDirExists  bool
	}{
		{
			name:              "we should return false from deletion check if not at file system capacity threshold",
			deletionExpected:  false,
			triggerThreshold:  .99,
			captureUsageRatio: .99,
		},
		{
			name:              "we return true from deletion check if at file system capacity threshold",
			deletionExpected:  true,
			triggerThreshold:  math.SmallestNonzeroFloat64,
			captureUsageRatio: math.SmallestNonzeroFloat64,
		},
		{
			name: "we return false from deletion check" +
				"if at file system capacity threshold but not capture dir threshold",
			deletionExpected:  false,
			triggerThreshold:  math.SmallestNonzeroFloat64,
			captureUsageRatio: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempCaptureDir := t.TempDir()
			// write testing files
			writeFiles(t, tempCaptureDir, []string{"1.capture", "2.capture"})
			// overwrite thresholds
			fsThresholdToTriggerDeletion := FSThresholdToTriggerDeletion
			captureDirToFSUsageRatio := CaptureDirToFSUsageRatio
			FSThresholdToTriggerDeletion = tc.triggerThreshold
			CaptureDirToFSUsageRatio = tc.captureUsageRatio
			t.Cleanup(func() {
				FSThresholdToTriggerDeletion = fsThresholdToTriggerDeletion
				CaptureDirToFSUsageRatio = captureDirToFSUsageRatio
			})
			logger := logging.NewTestLogger(t)
			usage, err := diskusage.Statfs(tempCaptureDir)
			test.That(t, err, test.ShouldBeNil)
			willDelete, err := shouldDeleteBasedOnDiskUsage(context.Background(), usage, tempCaptureDir, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, willDelete, test.ShouldEqual, tc.deletionExpected)
		})
	}
}

func writeFiles(t *testing.T, dir string, filenames []string) map[string]string {
	t.Helper()
	fileContents := []byte("never gonna let you down")
	filePaths := map[string]string{}
	for _, filename := range filenames {
		filePath := fmt.Sprintf("%s/%s", dir, filename)
		err := os.WriteFile(filePath, fileContents, 0o755)
		test.That(t, err, test.ShouldBeNil)
		filePaths[filename] = filePath
	}
	return filePaths
}

func getFileNames(t *testing.T, path string) []string {
	t.Helper()
	dirEntries, err := os.ReadDir(path)
	test.That(t, err, test.ShouldBeNil)
	output := []string{}
	for _, d := range dirEntries {
		output = append(output, d.Name())
	}
	return output
}
