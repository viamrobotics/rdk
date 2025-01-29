package sync

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

var (
	// FSThresholdToTriggerDeletion temporarily public for tests.
	FSThresholdToTriggerDeletion = .90
	// CaptureDirToFSUsageRatio temporarily public for tests.
	CaptureDirToFSUsageRatio = .5
)

var errAtSizeThreshold = errors.New("capture directory has reached or exceeded disk usage threshold for deletion")

func deleteExcessFilesOnSchedule(
	ctx context.Context,
	fileTracker *fileTracker,
	captureDir string,
	deleteEveryNth int,
	clock clock.Clock,
	logger logging.Logger,
) {
	if runtime.GOOS == "android" {
		logger.Debug("file deletion if disk is full is not currently supported on Android")
		return
	}
	t := clock.Ticker(CheckDeleteExcessFilesInterval)
	defer t.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-t.C:
			maybeDeleteExcessFiles(ctx, fileTracker, captureDir, deleteEveryNth, clock, logger)
		}
	}
}

func deleteExcessFiles(
	ctx context.Context,
	fileTracker *fileTracker,
	usage diskusage.DiskUsage,
	captureDir string,
	deleteEveryNth int,
	logger logging.Logger,
) (int, error) {
	shouldDelete, err := shouldDeleteBasedOnDiskUsage(
		ctx,
		usage,
		captureDir,
		logger)
	if err != nil {
		return 0, errors.Wrap(err, "error checking file system stats")
	}

	if !shouldDelete {
		return 0, nil
	}

	logger.Warnf("current disk usage of the data capture directory exceeds threshold (%f)", CaptureDirToFSUsageRatio)
	return deleteFiles(ctx, fileTracker, deleteEveryNth, captureDir, logger)
}

// returns false, nil if the threshold is not exceeded
// returns false, error if an IO error was encountered
// returns true, nil if the threshold is exceeded.
func exceedsDeletionThreshold(
	ctx context.Context,
	captureDirPath string,
	fileSystemSizeBytes float64,
	captureDirToFSUsageRatio float64,
) (bool, error) {
	var dirSize float64
	readSize := func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		dirSize += float64(fileInfo.Size())
		if dirSize/fileSystemSizeBytes >= captureDirToFSUsageRatio {
			return errAtSizeThreshold
		}
		return nil
	}

	err := filepath.WalkDir(captureDirPath, readSize)
	if err != nil {
		if errors.Is(err, errAtSizeThreshold) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// deleteFiles temporarily public for tests.
func deleteFiles(
	ctx context.Context,
	fileTracker *fileTracker,
	deleteEveryNth int,
	captureDirPath string,
	logger logging.Logger,
) (int, error) {
	index := 0
	deletedFileCount := 0
	logger.Infof("Deleting every %dth file", deleteEveryNth)
	fileDeletion := func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// this can happen if after we start walking the dir, the file changes from .prog to .capture
			// which throws a file not found error when we try to get the fileinfo. If we hit this, just
			// swallow the error and continue walking
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		if d.IsDir() {
			return nil
		}
		fileInfo, err := d.Info()
		if err != nil {
			// same reason as above
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		isCompletedDataCaptureFile := strings.Contains(fileInfo.Name(), data.CompletedCaptureFileExt)
		// if at nth file and the file is not currently being written, mark as in progress if possible
		if isCompletedDataCaptureFile && index%deleteEveryNth == 0 {
			if !fileTracker.markInProgress(path) {
				logger.Debugw("Tried to mark file as in progress but lock already held", "file", d.Name())
				return nil
			}
			if err := os.Remove(path); err != nil {
				logger.Warnw("error deleting file", "error", err)
				fileTracker.unmarkInProgress(path)
				return err
			}
			logger.Infof("successfully deleted %s", d.Name())
			deletedFileCount++
		}
		// only increment on completed files
		if isCompletedDataCaptureFile {
			index++
		}
		return nil
	}
	err := filepath.WalkDir(captureDirPath, fileDeletion)
	return deletedFileCount, err
}
