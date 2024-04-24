package builtin

import (
	"errors"
	"io/fs"
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
			// logger.Warn(fileInfo.Name())
			dirSize += fileInfo.Size()
			if float64(dirSize)/fsSize < captureDirRatioThreshold {
				logger.Warnw("At threshold to delete, going to delete", "size", float64(dirSize)/fsSize, "threshold", captureDirRatioThreshold)
				return errAtSizeThreshold
			} //else {
			// 	logger.Warnw("Not at threshold", "size", float64(dirSize)/fsSize, "threshold", captureDirRatioThreshold)
			// }
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
			logger.Debug(fileInfo.Name())
			isFileInProgress := strings.Contains(fileInfo.Name(), ".prog")
			// if at nth file, the file is not currenlty being written, the syncer isnt nil and isnt uploading the file or there is no syncer
			if index%n == 0 && !isFileInProgress && ((syncer != nil && syncer.MarkInProgress(path)) || syncer == nil) {
				logger.Debugw("Deleting file ", "name", fileInfo.Name())
				// err := os.Remove(path)
				if err != nil {
					logger.Debugw("error deleting file", "error", err)
					return err
				}
			}
			// only increment on completed files
			if !isFileInProgress {
				index++
			}
		}
		return nil
	}
	return filepath.WalkDir(captureDirPath, delete)
}
