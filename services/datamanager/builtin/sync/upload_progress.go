package sync

import (
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// UploadProgressLogInterval is the minimum duration between Info-level progress logs
// emitted while a file is uploading.
var UploadProgressLogInterval = 30 * time.Second

// uploadProgressLogger emits human-readable progress logs while a file
// upload stream is in flight. Each instance tracks a single upload attempt and is
// used by a single goroutine; it is not safe for concurrent use.
type uploadProgressLogger struct {
	logger         logging.Logger
	clock          clock.Clock
	path           string
	totalBytes     uint64
	sentBytes      uint64
	startTime      time.Time
	lastLog        time.Time
	loggedProgress bool
}

func newUploadProgressLogger(logger logging.Logger, clk clock.Clock, path string, totalBytes int64) *uploadProgressLogger {
	now := clk.Now()
	return &uploadProgressLogger{
		logger:     logger,
		clock:      clk,
		path:       path,
		totalBytes: uint64(max(totalBytes, 0)),
		startTime:  now,
		lastLog:    now,
	}
}

// addBytes records that n more bytes were sent on the stream, and logs progress at
// Info level if at least UploadProgressLogInterval has passed since the last progress
// log (or since the upload started).
func (p *uploadProgressLogger) addBytes(n int) {
	if p == nil {
		return
	}
	p.sentBytes += uint64(n)
	now := p.clock.Now()
	if now.Sub(p.lastLog) < UploadProgressLogInterval {
		return
	}
	p.lastLog = now
	p.loggedProgress = true
	p.logger.Infof("uploading %s: %s / %s (%s), rate: %s/s",
		p.path,
		utils.FormatBytes(p.sentBytes),
		utils.FormatBytes(p.totalBytes),
		p.percent(),
		utils.FormatBytes(p.rate(now)),
	)
}

// complete logs a summary of the finished upload. The summary logs at Info only if a
// progress line was already emitted; otherwise it logs at Debug so small, fast
// uploads stay quiet at the default level.
func (p *uploadProgressLogger) complete() {
	if p == nil {
		return
	}
	now := p.clock.Now()
	msg := fmt.Sprintf("uploaded %s: %s in %s (avg rate: %s/s)",
		p.path,
		utils.FormatBytes(p.sentBytes),
		now.Sub(p.startTime).Round(time.Millisecond),
		utils.FormatBytes(p.rate(now)),
	)
	if p.loggedProgress {
		p.logger.Info(msg)
	} else {
		p.logger.Debug(msg)
	}
}

// percent renders sentBytes/totalBytes as a percentage string.
func (p *uploadProgressLogger) percent() string {
	if p.totalBytes == 0 {
		return "?%"
	}
	return fmt.Sprintf("%d%%", p.sentBytes*100/p.totalBytes)
}

// rate returns the average upload rate in bytes/sec since the upload started.
func (p *uploadProgressLogger) rate(now time.Time) uint64 {
	elapsed := now.Sub(p.startTime).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return uint64(float64(p.sentBytes) / elapsed)
}
