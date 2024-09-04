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

type fileCountLogger struct {
	logger  logging.Logger
	workers *goutils.StoppableWorkers
}

func newFileCountLogger(logger logging.Logger) *fileCountLogger {
	return &fileCountLogger{
		logger:  logger,
		workers: goutils.NewBackgroundStoppableWorkers(),
	}
}

func (poller *fileCountLogger) reconfigure(captureDir string) {
	poller.workers.Stop()
	poller.workers = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
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
	poller.workers.Stop()
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
		// this is intentionally not doing as many checkas as getAllFilesToSync because
		// this is intended for debugging and does not need to be 100% accurate.
		isCompletedCaptureFile := filepath.Ext(path) == data.CompletedCaptureFileExt
		if isCompletedCaptureFile {
			numFiles++
		}
		return nil
	}))
	return numFiles
}
