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

// diskSpaceMonitor periodically checks the free space on the volume holding the
// packages directory and logs a warning when the volume is at or above
// diskusage.MaxUsedFraction utilization or has less than diskusage.MinFreeBytes free.
type diskSpaceMonitor struct {
	// path is any path on the volume we want to monitor. Statfs reports usage for
	// the whole volume containing this path, not just the directory itself.
	path   string
	logger logging.Logger
	worker *goutils.StoppableWorkers
}

// newDiskSpaceMonitor starts a background worker that checks free disk space
// immediately and then every diskSpaceCheckInterval. Call stop to shut it down.
//
// If path is empty there is no volume to monitor (this happens when a localRobot is
// built without the entrypoint defaulting that fills in PackagePath), so we return
// nil rather than spawning a worker that would log a "could not check" error every
// interval. stop() is nil-safe so callers don't need to special-case this.
func newDiskSpaceMonitor(path string, logger logging.Logger) *diskSpaceMonitor {
	if path == "" {
		logger.Debug("no package path configured; disk space monitor disabled")
		return nil
	}
	m := &diskSpaceMonitor{path: path, logger: logger}
	m.worker = goutils.NewBackgroundStoppableWorkers(m.run)
	return m
}

// run checks once up front so a low-space machine warns at startup rather than
// after a full interval, then checks every diskSpaceCheckInterval until ctx is done.
func (m *diskSpaceMonitor) run(ctx context.Context) {
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
}

func (m *diskSpaceMonitor) check(ctx context.Context) {
	// Statfs is a blocking syscall with no context support and can hang on an
	// unresponsive network mount. Run it off the worker goroutine so a hung call
	// doesn't wedge shutdown: if ctx is canceled first we return; the (buffered)
	// goroutine still completes its send and exits rather than blocking Stop.
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
	usedPercent := (1 - res.usage.AvailablePercent()) * 100
	if res.low {
		m.logger.Warnw("low free disk space",
			"path", m.path,
			"available", diskusage.FormatBytes(res.usage.AvailableBytes),
			"used_percent", fmt.Sprintf("%.1f%%", usedPercent),
			"threshold", fmt.Sprintf("%.0f%% used or <%s free",
				diskusage.MaxUsedFraction*100, diskusage.FormatBytes(diskusage.MinFreeBytes)))
	} else {
		// Logged at debug so a healthy machine doesn't emit a line every interval forever;
		// the low-space case above is what we actually want operators to see.
		m.logger.Debugw("free disk space",
			"path", m.path,
			"available", diskusage.FormatBytes(res.usage.AvailableBytes),
			"used_percent", fmt.Sprintf("%.1f%%", usedPercent))
	}
}

func (m *diskSpaceMonitor) stop() {
	if m == nil || m.worker == nil {
		return
	}
	m.worker.Stop()
}
