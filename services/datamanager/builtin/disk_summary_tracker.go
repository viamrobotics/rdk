package builtin

import (
	"context"
	"sync"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

// diskSummaryTracker runs in the background and periodically (every minute) collects
// disk usage stats, which are reported to FTDC.
type diskSummaryTracker struct {
	diskSummary diskSummary
	logger      logging.Logger
	mu          sync.Mutex
	worker      *goutils.StoppableWorkers

	// Sync config fields used for stale data warnings.
	syncIntervalMins float64
	shouldSync       func(context.Context) bool
	lastStaleWarning time.Time
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

const (
	diskSummaryTrackerInterval = 1 * time.Minute
	// minStaleThreshold is the minimum age of the oldest file before we consider data stale.
	minStaleThreshold = 3 * time.Minute
	// staleWarningInterval is the minimum time between consecutive stale data warnings.
	staleWarningInterval = 5 * time.Minute
)

func newDiskSummaryTracker(logger logging.Logger) *diskSummaryTracker {
	return &diskSummaryTracker{
		logger: logger,
	}
}

func (poller *diskSummaryTracker) reconfigure(dirs []string, syncIntervalMins float64, shouldSync func(context.Context) bool) {
	if poller.worker != nil {
		poller.worker.Stop()
	}

	poller.syncIntervalMins = syncIntervalMins
	poller.shouldSync = shouldSync
	poller.lastStaleWarning = time.Time{}

	poller.logger.Debug("datamanager disk state summary tracker running...")
	// Calculate and set the initial summary.
	poller.calculateAndSetSummary(context.Background(), dirs)

	poller.worker = goutils.NewStoppableWorkerWithTicker(
		diskSummaryTrackerInterval,
		func(ctx context.Context) {
			poller.calculateAndSetSummary(ctx, dirs)
		},
	)
}

func (poller *diskSummaryTracker) close() {
	if poller.worker != nil {
		poller.worker.Stop()
	}
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
		// Log any errors from the directory summary.
		if summary.Err != nil {
			poller.logger.Debugw("error getting directory summary", "path", summary.Path, "error", summary.Err)
		}

		// The first directory is the main capture directory.
		if i == 0 {
			usage, err := diskusage.Statfs(summary.Path)
			if err != nil {
				poller.logger.Debugw("failed to get disk usage stats", "path", summary.Path, "error", err)
			} else {
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

	poller.checkAndLogStaleData(ctx, earliestTime, totalFiles, totalBytes)
	poller.setSummary(diskSummary)
}

// checkAndLogStaleData logs a rate-limited message if the oldest file in the capture directory
// is significantly older than expected given the sync interval. Logs at WARN if sync should be
// actively happening (scheduler enabled and sync sensor allows it), or at DEBUG if sync is
// paused (e.g. selective sync sensor returned false).
func (poller *diskSummaryTracker) checkAndLogStaleData(ctx context.Context, earliestTime *time.Time, totalFiles, totalBytes int64) {
	if earliestTime == nil || poller.shouldSync == nil {
		return
	}

	staleThreshold := time.Duration(10 * poller.syncIntervalMins * float64(time.Minute))
	if staleThreshold < minStaleThreshold {
		staleThreshold = minStaleThreshold
	}

	age := time.Since(*earliestTime)
	if age <= staleThreshold {
		return
	}

	now := time.Now()
	if !poller.lastStaleWarning.IsZero() && now.Sub(poller.lastStaleWarning) < staleWarningInterval {
		return
	}
	poller.lastStaleWarning = now

	msg := "Capture data may not be syncing: oldest file is %s old, expected less than %s. " +
		"There are %d files (%s) waiting to sync. " +
		"Data may be generating faster than it can be uploaded, or uploads may be failing."

	if poller.shouldSync(ctx) {
		poller.logger.Warnf(msg,
			age.Round(time.Second), staleThreshold.Round(time.Second),
			totalFiles, data.FormatBytesI64(totalBytes),
		)
	} else {
		poller.logger.Debugf(msg,
			age.Round(time.Second), staleThreshold.Round(time.Second),
			totalFiles, data.FormatBytesI64(totalBytes),
		)
	}
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
