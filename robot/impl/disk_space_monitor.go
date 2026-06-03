package robotimpl

import (
	"context"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/diskusage"
)

// diskSpaceCheckInterval is how often viam-server checks free disk space.
const diskSpaceCheckInterval = 5 * time.Minute

// diskSpaceMonitor periodically checks the free space on the volume holding the
// packages directory and logs a warning when it drops below diskusage.MinFreeBytes.
type diskSpaceMonitor struct {
	// path is any path on the volume we want to monitor. Statfs reports usage for
	// the whole volume containing this path, not just the directory itself.
	path   string
	logger logging.Logger
	worker *goutils.StoppableWorkers
}

// newDiskSpaceMonitor starts a background worker that checks free disk space
// immediately and then every diskSpaceCheckInterval. Call stop to shut it down.
func newDiskSpaceMonitor(path string, logger logging.Logger) *diskSpaceMonitor {
	m := &diskSpaceMonitor{path: path, logger: logger}
	// Check once up front so a low-space machine warns at startup rather than after
	// a full interval.
	m.check(context.Background())
	m.worker = goutils.NewStoppableWorkerWithTicker(diskSpaceCheckInterval, m.check)
	return m
}

func (m *diskSpaceMonitor) check(_ context.Context) {
	enough, available, err := diskusage.EnoughFreeSpace(m.path, diskusage.MinFreeBytes)
	if err != nil {
		m.logger.Debugw("could not check free disk space", "path", m.path, "error", err)
		return
	}
	if enough {
		m.logger.Infow("free disk space", "path", m.path, "available", diskusage.FormatBytes(available))
	} else {
		m.logger.Warnw("low free disk space",
			"path", m.path,
			"available", diskusage.FormatBytes(available),
			"threshold", diskusage.FormatBytes(diskusage.MinFreeBytes))
	}
}

func (m *diskSpaceMonitor) stop() {
	if m.worker != nil {
		m.worker.Stop()
	}
}
