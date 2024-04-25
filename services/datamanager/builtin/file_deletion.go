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
	fileDeletionThreshold    = .2
	captureDirRatioThreshold = .1
	n                        = 4
)

var errAtSizeThreshold = errors.New("capture dir is at correct size")

func checkFileSystemStats(ctx context.Context, captureDirPath string, logger logging.Logger) (bool, error) {
	usage := du.NewDiskUsage(captureDirPath)
	usedSpace := usage.Usage()
	if math.IsNaN(float64(usedSpace)) {
		return false, nil
	}
	if usedSpace < fileDeletionThreshold {
		logger.Warn("disk not full enough, exiting")
		return false, nil
	}
	//walk the dir to get capture stats
	return checkCaptureDirSize(ctx, captureDirPath, float64(usage.Size()), logger)
}

func checkCaptureDirSize(ctx context.Context, captureDirPath string, fsSize float64, logger logging.Logger) (bool, error) {
	var dirSize int64 = 0

	readSize := func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
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
			if float64(dirSize)/fsSize > captureDirRatioThreshold {
				logger.Warnw("At threshold to delete, going to delete", "size", float64(dirSize)/fsSize, "threshold", captureDirRatioThreshold)
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
		logger.Warnw("Not at threshold", "size", float64(dirSize)/fsSize, "threshold", captureDirRatioThreshold)

	}
	return err != nil && errors.Is(err, errAtSizeThreshold), nil
}

func deleteFiles(ctx context.Context, syncer datasync.Manager, captureDirPath string, logger logging.Logger) error {
	index := 0
	deletedFileCount := 0
	delete := func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return err
		}
		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			isFileInProgress := strings.Contains(fileInfo.Name(), datacapture.InProgressFileExt)
			// if at nth file, the file is not currenlty being written, the syncer isnt nil and isnt uploading the file or there is no syncer
			if index%n == 0 && !isFileInProgress && ((syncer != nil && syncer.MarkInProgress(path)) || syncer == nil) {
				logger.Debugw("Deleting file ", "name", fileInfo.Name())
				err := os.Remove(path)
				if err != nil {
					logger.Warnw("error deleting file", "error", err)
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
