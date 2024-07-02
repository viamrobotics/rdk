package builtin

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/utils/diskusage"
)

var (
	fsThresholdToTriggerDeletion = .90
	captureDirToFSUsageRatio     = .5
	defaultDeleteEveryNth        = 5
)

var errAtSizeThreshold = errors.New("capture directory has reached or exceeded disk usage threshold for deletion")

func shouldDeleteBasedOnDiskUsage(ctx context.Context, captureDirPath string, logger logging.Logger) (bool, error) {
	usage := diskusage.NewDiskUsage(captureDirPath)
	// we get usage this way to ensure we get the amount of remaining space in the partition.
	// calling usage.Usage() returns the usage of the whole disk, not the user partition
	usedSpace := 1.0 - float64(usage.Available())/float64(usage.Size())
	if math.IsNaN(usedSpace) {
		return false, nil
	}
	if usedSpace < fsThresholdToTriggerDeletion {
		logger.Debugf("disk not full enough, exiting. Used space: %f, available space: %d, size: %d",
			usedSpace, usage.Available(), usage.Size())
		return false, nil
	}
	// Walk the dir to get capture stats
	shouldDelete, err := exceedsDeletionThreshold(ctx, captureDirPath, float64(usage.Size()), logger)
	if err != nil && !shouldDelete {
		logger.Warnf("Disk nearing capacity but data capture directory is below %f of that size, file deletion will not run",
			captureDirToFSUsageRatio)
	}
	return shouldDelete, err
}

func exceedsDeletionThreshold(ctx context.Context, captureDirPath string, fsSize float64, logger logging.Logger) (bool, error) {
	var dirSize int64

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

		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			dirSize += fileInfo.Size()
			if float64(dirSize)/fsSize >= captureDirToFSUsageRatio {
				logger.Warnf("current disk usage of the data capture directory (%f) exceeds threshold (%f)",
					float64(dirSize)/fsSize, captureDirToFSUsageRatio)
				return errAtSizeThreshold
			}
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

func deleteFiles(ctx context.Context, syncer datasync.Manager, deleteEveryNth int,
	captureDirPath string, logger logging.Logger,
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

		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				// same reason as above
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			isCompletedDataCaptureFile := strings.Contains(fileInfo.Name(), datacapture.FileExt)
			// if at nth file and the file is not currently being written, mark as in progress if possible
			if isCompletedDataCaptureFile && index%deleteEveryNth == 0 {
				if syncer != nil && !syncer.MarkInProgress(path) {
					logger.Debugw("Tried to mark file as in progress but lock already held", "file", d.Name())
					return nil
				}
				if err := os.Remove(path); err != nil {
					logger.Warnw("error deleting file", "error", err)
					if syncer != nil {
						syncer.UnmarkInProgress(path)
					}
					return err
				}
				logger.Infof("successfully deleted %s", d.Name())
				deletedFileCount++
			}
			// only increment on completed files
			if isCompletedDataCaptureFile {
				index++
			}
		}
		return nil
	}
	err := filepath.WalkDir(captureDirPath, fileDeletion)
	return deletedFileCount, err
}
