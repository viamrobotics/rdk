package sync

import (
	"context"

	"github.com/benbjohnson/clock"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

func maybeDeleteExcessFiles(
	ctx context.Context,
	fileTracker *fileTracker,
	captureDir string,
	deleteEveryNth int,
	clock clock.Clock,
	logger logging.Logger,
) {
	start := clock.Now()
	deletedFileCount, err := deleteExcessFiles(
		ctx,
		fileTracker,
		diskusage.DiskUsage{},
		captureDir,
		deleteEveryNth,
		logger)

	duration := clock.Since(start)

	switch {
	case err != nil:
		logger.Errorw("error deleting cached datacapture files", "error", err, "execution time", duration.String())
	case deletedFileCount > 0:
		logger.Infof("%d files have been deleted to avoid the disk filling up, execution time: %s", deletedFileCount, duration.String())
	default:
		logger.Infof("no files deleted, execution time: %s", duration)
	}
}

func shouldDeleteBasedOnDiskUsage(
	ctx context.Context,
	usage diskusage.DiskUsage,
	captureDirPath string,
	logger logging.Logger,
) (bool, error) {
	return false, nil
}
