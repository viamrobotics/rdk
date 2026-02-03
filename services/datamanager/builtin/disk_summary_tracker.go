package builtin

import (
	"context"
	"sync"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

type diskSummaryTracker struct {
	diskSummary diskSummary
	logger      logging.Logger
	mu          sync.Mutex
	worker      *goutils.StoppableWorkers
}

type diskSummary struct {
	DiskUsage             diskUsageSummary
	OldestCaptureFileTime *time.Time
	SyncPaths             syncPathsSummary
}

type diskUsageSummary struct {
	AvailableGB      float64
	SizeGB           float64
	AvailablePercent float64
}

type syncPathsSummary struct {
	TotalFiles     int64
	TotalSizeBytes int64
}

const diskSummaryTrackerInterval = 1 * time.Minute

func newDiskSummaryTracker(logger logging.Logger) *diskSummaryTracker {
	return &diskSummaryTracker{
		logger: logger,
		worker: goutils.NewBackgroundStoppableWorkers(),
	}
}

func (poller *diskSummaryTracker) reconfigure(dirs []string) {
	poller.worker.Stop()
	poller.worker = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		poller.logger.Debug("datamanager disk state summary tracker starting...")

		// Calculate and set the initial summary.
		poller.calculateAndSetSummary(ctx, dirs)

		t := time.NewTicker(diskSummaryTrackerInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				poller.calculateAndSetSummary(ctx, dirs)
			}
		}
	})
}

func (poller *diskSummaryTracker) close() {
	poller.worker.Stop()
}

func (poller *diskSummaryTracker) calculateAndSetSummary(ctx context.Context, dirs []string) {
	diskSummary := diskSummary{}
	var summaries []DirSummary
	for _, dir := range dirs {
		summaries = append(summaries, DiskSummary(ctx, dir)...)
		if ctx.Err() != nil {
			return
		}
	}

	// Accumulate totals across all summaries.
	var totalFiles int64
	var totalBytes int64
	var earliestTime *time.Time

	for i, summary := range summaries {
		// The first directory is the main capture directory.
		if i == 0 {
			usage, err := diskusage.Statfs(summary.Path)
			if err == nil {
				diskSummary.DiskUsage.AvailableGB = float64(usage.AvailableBytes) / (1 << 30)
				diskSummary.DiskUsage.SizeGB = float64(usage.SizeBytes) / (1 << 30)
				diskSummary.DiskUsage.AvailablePercent = usage.AvailablePercent() * 100
			}
		}

		// Accumulate file counts and sizes.
		totalFiles += summary.FileCount
		totalBytes += summary.FileSize

		// Track earliest file time.
		if summary.DataTimeRange != nil {
			if earliestTime == nil || summary.DataTimeRange.Start.Before(*earliestTime) {
				earliestTime = &summary.DataTimeRange.Start
			}
		}
	}

	diskSummary.SyncPaths.TotalFiles = totalFiles
	diskSummary.SyncPaths.TotalSizeBytes = totalBytes
	diskSummary.OldestCaptureFileTime = earliestTime

	poller.setSummary(diskSummary)
}

func (poller *diskSummaryTracker) getSummary() diskSummary {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	return poller.diskSummary
}

func (poller *diskSummaryTracker) setSummary(diskSummary diskSummary) {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	poller.diskSummary = diskSummary
}
