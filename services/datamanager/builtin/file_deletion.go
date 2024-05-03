package builtin

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricochet2200/go-disk-usage/du"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
)

// TODO change these values back to what they should be.
var (
	fsThresholdToTriggerDeletion = .95
	captureDirToFSUsageRatio     = .5
	deleteEveryNth               = 4
)

var errAtSizeThreshold = errors.New("capture dir is at correct size")

func shouldDeleteBasedOnDiskUsage(ctx context.Context, captureDirPath string, logger logging.Logger) (bool, error) {
	usage := du.NewDiskUsage(captureDirPath)
	// we get usage this way to ensure we get the amount of remaining space in the partition.
	// calling usage.Usage() returns the usage of the whole disk, not the user partition
	usedSpace := 1.0 - float64(usage.Available())/float64(usage.Size())
	if math.IsNaN(usedSpace) {
		return false, nil
	}
	if usedSpace < fsThresholdToTriggerDeletion {
		logger.Warnf("disk not full enough, exiting used space: %f", usedSpace)
		return false, nil
	}
	// Walk the dir to get capture stats
	return exceedsDeletionThreshold(ctx, captureDirPath, float64(usage.Size()), logger)
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
				return err
			}
			dirSize += fileInfo.Size()
			if float64(dirSize)/fsSize >= captureDirToFSUsageRatio {
				logger.Warnf("At threshold to delete, going to delete usage ratio %f threshold %d", float64(dirSize)/fsSize, captureDirToFSUsageRatio)
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
	logger.Warnf("Not at threshold size %f threshold %f", float64(dirSize)/fsSize, captureDirToFSUsageRatio)
	return false, nil
}

func deleteFiles(ctx context.Context, syncer datasync.Manager, captureDirPath string, logger logging.Logger) (int, error) {
	index := 0
	deletedFileCount := 0
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
			isFileInProgress := strings.Contains(fileInfo.Name(), datacapture.InProgressFileExt)
			// if at nth file and the file is not currently being written, mark as in progress if possible
			if index%deleteEveryNth == 0 && !isFileInProgress {
				if syncer != nil && !syncer.MarkInProgress(path) {
					logger.Warnw("Tried to mark file as in progress but lock already held", "file", d.Name())
					return nil
				}
				if err := os.Remove(path); err != nil {
					logger.Warnw("error deleting file", "error", err)
					if syncer != nil {
						syncer.UnmarkInProgress(path)
					}
					return err
				}
				deletedFileCount++
			}
			// only increment on completed files
			if !isFileInProgress {
				index++
			}
		}
		return nil
	}
	err := filepath.WalkDir(captureDirPath, fileDeletion)
	return deletedFileCount, err
}
