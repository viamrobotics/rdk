package builtin

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/rdk/logging"

	"github.com/ricochet2200/go-disk-usage/du"
	"go.viam.com/rdk/services/datamanager/datasync"
)

const (
	fileDeletionThreshold    = .95
	captureDirRatioThreshold = .5
	n                        = 4
)

var errAtSizeThreshold = errors.New("capture dir is at correct size")

func checkFileSystemStats(captureDirPath string, logger logging.Logger) (bool, error) {
	usage := du.NewDiskUsage(captureDirPath)
	usedSpace := usage.Usage()
	if usedSpace < fileDeletionThreshold {
		logger.Debug("Should exit thread, disk not full enough ignoring for now")
		// return false, nil
	}
	//walk the dir to get capture stats
	return checkCaptureDirSize(captureDirPath, float64(usage.Size()), logger)
}

func checkCaptureDirSize(captureDirPath string, fsSize float64, logger logging.Logger) (bool, error) {
	var dirSize int64 = 0
	logger.Warn("checking size")

	readSize := func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			logger.Warn(fileInfo.Name())
			dirSize += fileInfo.Size()
			if float64(dirSize)/fsSize < captureDirRatioThreshold {
				logger.Warnw("At threshold to delete, going to delete", "size", float64(dirSize)/fsSize, "threshold", captureDirRatioThreshold)
				return errAtSizeThreshold
			} else {
				logger.Warnw("Not at threshold", "size", float64(dirSize)/fsSize, "threshold", captureDirRatioThreshold)
			}
		}
		return nil
	}

	err := filepath.WalkDir(captureDirPath, readSize)
	if err != nil && !errors.Is(err, errAtSizeThreshold) {
		return false, err
	}
	return err != nil && errors.Is(err, errAtSizeThreshold), nil
}

func deleteFiles(syncer datasync.Manager, captureDirPath string, logger logging.Logger) error {
	index := 0
	delete := func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			// if its the nth file, is not currently in progress of being synced and is not currently being written to by datacapture
			logger.Debug(fileInfo.Name())
			isFileInProgress := strings.Contains(fileInfo.Name(), ".prog")
			if index != 0 && index%n == 0 && !isFileInProgress {
				//mark path as inprogress if syncer is not nil
				if (syncer != nil && syncer.MarkInProgress(fileInfo.Name())) || syncer == nil {
					logger.Debugw("Would delete file (not doing currently)", "file name", path)
					logger.Debug(path)
					err := os.Remove(path)
					if err != nil {
						logger.Debugw("error deleting file", "error", err)
						return err
					}
				}

			}
			// only increment on completed files
			if !isFileInProgress {
				index++
			}
		}
		return nil
	}

	err := filepath.WalkDir(captureDirPath, delete)
	return err
}
