package robotimpl

import (
	"context"
	"fmt"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

// diskSpaceCheckInterval is how often viam-server checks free disk space.
const diskSpaceCheckInterval = 5 * time.Minute

// diskSpaceMonitor periodically logs a warning when the volume holding the packages directory
// is low on space (see diskusage.IsLow). It watches only the packages directory's volume — not
// the data-capture dir, which is the likelier disk-filler and may live on a different volume; the
// data manager tracks that separately.
type diskSpaceMonitor struct {
	// path is any path on the monitored volume; Statfs reports usage for the whole volume.
	path   string
	logger logging.Logger
	worker *goutils.StoppableWorkers
}

// newDiskSpaceMonitor starts a background worker that checks free disk space immediately and
// then every diskSpaceCheckInterval; call stop to shut it down. Returns nil when path is empty
// (no volume to monitor); stop() is nil-safe so callers needn't special-case that.
func newDiskSpaceMonitor(path string, logger logging.Logger) *diskSpaceMonitor {
	if path == "" {
		logger.Debug("no package path to watch; disk space monitor disabled")
		return nil
	}
	m := &diskSpaceMonitor{path: path, logger: logger}
	// Run on a cancellable background worker so a hung network mount can't wedge startup, and run
	// the first check immediately rather than after a full interval.
	m.worker = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		m.check(ctx)
		ticker := time.NewTicker(diskSpaceCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.check(ctx)
			}
		}
	})
	return m
}

func (m *diskSpaceMonitor) check(ctx context.Context) {
	// Statfs is an uncancellable syscall that can hang on a dead mount, so run it on a throwaway
	// goroutine and select against ctx: if ctx is canceled first we return without waiting and Stop
	// never blocks. resCh is buffered so that goroutine can finish its send and exit even after we
	// return on cancel; only a permanently-hung statfs leaks a goroutine, and the worker stops
	// ticking once canceled so at most one ever leaks.
	type result struct {
		usage diskusage.DiskUsage
		low   bool
		err   error
	}
	resCh := make(chan result, 1)
	goutils.PanicCapturingGo(func() {
		usage, low, err := diskusage.IsLowOnSpace(m.path)
		resCh <- result{usage: usage, low: low, err: err}
	})

	var res result
	select {
	case <-ctx.Done():
		return
	case res = <-resCh:
	}

	if res.err != nil {
		m.logger.Debugw("could not check free disk space", "path", m.path, "error", res.err)
		return
	}
	// Pseudo-filesystems can report SizeBytes == 0, making AvailablePercent NaN; show "unknown"
	// rather than "NaN%".
	usedPercent := "unknown"
	if res.usage.SizeBytes > 0 {
		usedPercent = fmt.Sprintf("%.1f%%", (1-res.usage.AvailablePercent())*100)
	}
	if res.low {
		m.logger.Warnw("low free disk space",
			"path", m.path,
			"available", diskusage.FormatBytes(res.usage.AvailableBytes),
			"used_percent", usedPercent,
			"threshold", fmt.Sprintf("%.0f%% used or <%s free",
				diskusage.MaxUsedFraction*100, diskusage.FormatBytes(diskusage.MinFreeBytes)))
	} else {
		// Debug so a healthy machine doesn't log every interval; the low-space case above is the signal.
		m.logger.Debugw("free disk space",
			"path", m.path,
			"available", diskusage.FormatBytes(res.usage.AvailableBytes),
			"used_percent", usedPercent)
	}
}

func (m *diskSpaceMonitor) stop() {
	if m == nil || m.worker == nil {
		return
	}
	m.worker.Stop()
}
