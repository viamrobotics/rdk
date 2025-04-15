//go:build !windows

package sync

import (
	"context"
	"fmt"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

func maybeDeleteExcessFiles(
	ctx context.Context,
	fileTracker *fileTracker,
	captureDir string,
	deleteEveryNth int,
	diskUsageThreshold float64,
	captureDirToFSThreshold float64,
	clock clock.Clock,
	logger logging.Logger,
) {
	start := clock.Now()
	logger.Debug("checking disk usage")
	usage, err := diskusage.Statfs(captureDir)
	logger.Debugf("disk usage: %s", usage)
	if err != nil {
		logger.Error(errors.Wrap(err, "error checking file system stats"))
		return
	}

	if usage.SizeBytes == 0 {
		logger.Error("captureDir partition has size zero")
		return
	}
	deletedFileCount, err := deleteExcessFiles(
		ctx,
		fileTracker,
		usage,
		captureDir,
		deleteEveryNth,
		diskUsageThreshold,
		captureDirToFSThreshold,
		logger)

	duration := clock.Since(start)

	switch {
	case err != nil:
		logger.Errorw("error deleting cached datacapture files", "error", err, "execution time", duration.String())
	case deletedFileCount > 0:
		logger.Infof("%d files have been deleted to avoid the disk filling up, execution time: %s", deletedFileCount, duration.String())
	default:
		logger.Debugf("no files deleted, execution time: %s", duration)
	}
}

func shouldDeleteBasedOnDiskUsage(
	ctx context.Context,
	usage diskusage.DiskUsage,
	captureDirPath string,
	diskUsageThreshold float64,
	captureDirToFSThreshold float64,
	logger logging.Logger,
) (bool, error) {
	usedSpace := 1.0 - usage.AvailablePercent()
	if usedSpace < diskUsageThreshold {
		logger.Debugf("disk not full enough. Threshold: %s, Used space: %s, %s",
			fmt.Sprintf("%.2f", diskUsageThreshold*100)+"%",
			fmt.Sprintf("%.2f", usedSpace*100)+"%",
			usage)
		return false, nil
	}
	// Walk the dir to get capture stats
	shouldDelete, err := exceedsDeletionThreshold(
		ctx,
		captureDirPath,
		float64(usage.SizeBytes),
		captureDirToFSThreshold,
	)
	if err != nil && !shouldDelete {
		logger.Warnf("Disk nearing capacity but data capture directory is below %f of that size, file deletion will not run",
			captureDirToFSThreshold)
	}
	return shouldDelete, err
}
