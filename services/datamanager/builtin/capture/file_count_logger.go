package capture

import (
	"context"
	"os"
	"path/filepath"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
)

// fileCountLogger logs the number of completed capture files that
// are eligible to be synced in the capture directory.
// It excludes files in failed directories
// This is a temporary measure to provide some imperfect signal into what
// data capture files are eligible to be synced on the robot.
type fileCountLogger struct {
	logger logging.Logger
	worker *goutils.StoppableWorkers
}

func newFileCountLogger(logger logging.Logger) *fileCountLogger {
	return &fileCountLogger{
		logger: logger,
		worker: goutils.NewBackgroundStoppableWorkers(),
	}
}

func (poller *fileCountLogger) reconfigure(captureDir string) {
	poller.worker.Stop()
	poller.worker = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		t := time.NewTicker(captureDirSizeLogInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				numFiles := countFiles(ctx, captureDir)
				if numFiles > minNumFiles {
					poller.logger.Infof("Capture dir contains %d files", numFiles)
				}
			}
		}
	})
}

func (poller *fileCountLogger) close() {
	poller.worker.Stop()
}

func countFiles(ctx context.Context, captureDir string) int {
	numFiles := 0
	goutils.UncheckedError(filepath.Walk(captureDir, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		//nolint:nilerr
		if err != nil {
			return nil
		}

		// Do not count the files in the corrupted data directory.
		if info.IsDir() && info.Name() == datasync.FailedDir {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}
		// this is intentionally not doing as many checks as getAllFilesToSync because
		// this is intended for debugging and does not need to be 100% accurate.
		isCompletedCaptureFile := filepath.Ext(path) == data.CompletedCaptureFileExt
		if isCompletedCaptureFile {
			numFiles++
		}
		return nil
	}))
	return numFiles
}
