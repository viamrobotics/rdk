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

	"github.com/ricochet2200/go-disk-usage/du"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
)

// TODO change these values back to what they should be
const (
	fsThresholdToTriggerDeletion = .8
	captureDirToFSUsageRatio     = .5
	n                            = 4
)

var errAtSizeThreshold = errors.New("capture dir is at correct size")

func shouldDeleteBasedOnDiskUsage(ctx context.Context, captureDirPath string, logger logging.Logger) (bool, error) {
	usage := du.NewDiskUsage(captureDirPath)
	usedSpace := usage.Usage()
	if math.IsNaN(float64(usedSpace)) {
		return false, nil
	}
	if usedSpace < fsThresholdToTriggerDeletion {
		logger.Warn("disk not full enough, exiting")
		return false, nil
	}
	// Walk the dir to get capture stats
	return exceedsDeletionThreshold(ctx, captureDirPath, float64(usage.Size()), logger)
}

func exceedsDeletionThreshold(ctx context.Context, captureDirPath string, fsSize float64, logger logging.Logger) (bool, error) {
	var dirSize int64 = 0

	readSize := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			dirSize += fileInfo.Size()
			if float64(dirSize)/fsSize > captureDirToFSUsageRatio {
				logger.Warnw("At threshold to delete, going to delete", "usage ratio", float64(dirSize)/fsSize, "threshold", captureDirToFSUsageRatio)
				return errAtSizeThreshold
			}
		}
		return nil
	}

	err := filepath.WalkDir(captureDirPath, readSize)
	if err != nil && !errors.Is(err, errAtSizeThreshold) {
		return false, err
	}
	if err == nil {
		logger.Warnw("Not at threshold", "size", float64(dirSize)/fsSize, "threshold", captureDirToFSUsageRatio)

	}
	return err != nil && errors.Is(err, errAtSizeThreshold), nil
}

func deleteFiles(ctx context.Context, syncer datasync.Manager, captureDirPath string, logger logging.Logger) error {
	index := 0
	deletedFileCount := 0
	delete := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			isFileInProgress := strings.Contains(fileInfo.Name(), datacapture.InProgressFileExt)
			// if at nth file and the file is not currently being written, mark as in progress if possible
			if index%n == 0 && !isFileInProgress {
				if syncer != nil && !syncer.MarkInProgress(path) {
					logger.Warnw("Tried to mark file as in progress but lock already held", "file", d.Name())
					index++
					return nil
				}
				logger.Debugw("Deleting file ", "name", fileInfo.Name())
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
	err := filepath.WalkDir(captureDirPath, delete)
	logger.Infof("%d files have been deleted to avoid the disk filling up", deletedFileCount)
	return err
}
