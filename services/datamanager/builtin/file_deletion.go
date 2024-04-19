package builtin

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/ricochet2200/go-disk-usage/du"
)

const fileDeletionThreshold = .95
const captureDirRatioThreshold = .5

var errAtSizeThreshold = errors.New("Capture dir is at correct size")

func checkFileSystemStats(captureDir string) (bool, error) {
	usage := du.NewDiskUsage(captureDir)
	usedSpace := usage.Usage()
	if usedSpace < fileDeletionThreshold {
		return false, nil
	}
	//walk the dir to get capture stats
	return checkCaptureDirSize(captureDir, usage.Size())

}

func checkCaptureDirSize(path string, fsSize uint64) (bool, error) {
	var dirSize uint64 = 0

	readSize := func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			dirSize += uint64(fileInfo.Size())
			if float64(dirSize/(fsSize)) > captureDirRatioThreshold {
				return errAtSizeThreshold
			}
		}
		return nil
	}

	err := filepath.WalkDir(path, readSize)
	if err != nil && !errors.Is(err, errAtSizeThreshold) {
		return false, err
	}
	return err != nil && errors.Is(err, errAtSizeThreshold), nil
}
