package sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// UploadProgressLogInterval is the interval at which in-flight upload progress is logged
// at Info level. A file whose upload completes within one interval logs only a single
// completion summary.
var UploadProgressLogInterval = 20 * time.Second

// uploadProgressLogger tracks the progress of a single in-flight file upload. While the
// upload is running a background goroutine logs progress at Info level once per
// UploadProgressLogInterval. The upload owns the logger's whole outcome through it: each
// chunk calls addBytes, and on termination the caller invokes onResult, which records a
// success (complete), a terminal failure (failure), or neither for outcomes that are not
// genuine failures — cancellation and "not ready" conditions, which resolve on a later sync.
// These update the per-data-type stats captured at creation so upload paths no longer
// scatter stat increments alongside the progress calls. close is deferred separately as a
// cleanup safety net and is idempotent with the other two.
//
// A nil *uploadProgressLogger is a valid no-op receiver so upload code paths that lack a
// logger (e.g. tests) need no special casing.
type uploadProgressLogger struct {
	logger     logging.Logger
	clock      clock.Clock
	path       string
	totalBytes uint64

	// wireBytes is the total bytes sent over the wire for this file, cumulative across all
	// retry attempts (re-sent chunks are counted again). It drives the throughput display
	// and the FTDC in-progress byte counter, and never decreases.
	wireBytes atomic.Uint64
	// attemptBytes is the bytes sent during the current attempt; startAttempt resets it to 0
	// so the percent display reflects the in-flight attempt and never exceeds 100% after a
	// retry re-sends from the beginning.
	attemptBytes atomic.Uint64
	// attempt is this file's own attempt count (1 on the first try, incremented per retry),
	// used only for display; distinct from the cumulative stats.uploadAttempts counter.
	attempt atomic.Uint64
	// All byte counters are written by the upload goroutine(s) and read by the progress
	// goroutine, so they must be accessed atomically.

	// stats holds the cumulative per-data-type FTDC counters that addBytes, startAttempt,
	// attemptFailed, complete, and failure update on this upload's behalf.
	stats *dataTypeUploadStats

	startTime time.Time
	workers   *goutils.StoppableWorkers
	stopOnce  sync.Once
}

// newUploadProgressLogger returns a logger that has already started its background
// progress goroutine. stats is required (non-nil): it is the per-data-type FTDC section
// this upload increments. The ticker is created synchronously before returning so that
// tests driving a mock clock cannot advance it before the goroutine begins waiting on it.
func newUploadProgressLogger(
	logger logging.Logger, clk clock.Clock, path string, totalBytes int64, stats *dataTypeUploadStats,
) *uploadProgressLogger {
	p := &uploadProgressLogger{
		logger:     logger,
		clock:      clk,
		path:       path,
		totalBytes: uint64(max(totalBytes, 0)),
		stats:      stats,
		startTime:  clk.Now(),
	}
	ticker := clk.Ticker(UploadProgressLogInterval)
	p.workers = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		p.run(ctx, ticker)
	})
	return p
}

// addBytes records that n more bytes were sent on the stream, updating the cumulative wire
// counter, the current-attempt counter, and the cumulative FTDC in-progress byte counter.
func (p *uploadProgressLogger) addBytes(n int) {
	if p == nil {
		return
	}
	p.wireBytes.Add(uint64(n))
	p.attemptBytes.Add(uint64(n))
	p.stats.uploadingBytes.Add(uint64(n))
}

// startAttempt marks the beginning of an upload attempt. It resets the per-attempt byte
// counter so the percent display restarts at 0 for a retried upload, and records the
// attempt in the FTDC stats. The reset is safe against the progress goroutine because
// attemptBytes only feeds the percent (a ratio, never a delta), so it cannot underflow.
func (p *uploadProgressLogger) startAttempt() {
	if p == nil {
		return
	}
	p.attemptBytes.Store(0)
	p.attempt.Add(1)
	p.stats.uploadAttempts.Add(1)
}

// attemptNum returns this file's current attempt number for display, defaulting to 1 for
// upload paths that don't (yet) call startAttempt so the log never reads "attempt 0".
func (p *uploadProgressLogger) attemptNum() uint64 {
	if n := p.attempt.Load(); n > 0 {
		return n
	}
	return 1
}

// attemptFailed records that an individual upload attempt returned an error, whether it
// will be retried or is terminal. It is distinct from failure, which records the file-level
// terminal outcome once retries are exhausted.
func (p *uploadProgressLogger) attemptFailed() {
	if p == nil {
		return
	}
	p.stats.uploadAttemptFailures.Add(1)
}

// run logs upload progress once per tick until the context is cancelled. The percent
// reflects the current attempt (attemptBytes), while the rates reflect the cumulative wire
// throughput (wireBytes), which is monotonic and so needs no reset handling here.
func (p *uploadProgressLogger) run(ctx context.Context, ticker *clock.Ticker) {
	defer ticker.Stop()
	lastTick := p.startTime
	var lastWire uint64
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			att := p.attemptBytes.Load()
			wire := p.wireBytes.Load()
			p.logger.Infof("Progress uploading %s (attempt %d, %s elapsed): %s / %s (%s), rate: %s/s (avg %s/s)",
				p.path,
				p.attemptNum(),
				now.Sub(p.startTime).Round(time.Millisecond),
				utils.FormatBytes(att),
				utils.FormatBytes(p.totalBytes),
				p.percent(att),
				utils.FormatBytes(rate(wire-lastWire, now.Sub(lastTick))),
				utils.FormatBytes(rate(wire, now.Sub(p.startTime))),
			)
			lastTick = now
			lastWire = wire
		}
	}
}

// complete stops the progress goroutine, logs a one-line success summary, and records the
// completed upload in the FTDC stats. It must only be called when the upload succeeded.
// bytesUploaded is the number of bytes actually sent, which can be 0 for a file dropped
// without uploading (e.g. an empty capture file); the summary still reports the file size.
func (p *uploadProgressLogger) complete(bytesUploaded uint64) {
	if p == nil {
		return
	}
	p.stop()
	now := p.clock.Now()
	elapsed := now.Sub(p.startTime)
	p.logger.Debugf("Completed upload %s (attempt %d): %s in %s (avg rate: %s/s)",
		p.path,
		p.attemptNum(),
		utils.FormatBytes(p.totalBytes),
		elapsed.Round(time.Millisecond),
		utils.FormatBytes(rate(p.totalBytes, elapsed)),
	)
	p.stats.uploadedFileCount.Add(1)
	p.stats.completedUploadBytes.Add(bytesUploaded)
}

// failure stops the progress goroutine and records a terminally failed upload in the FTDC
// stats. It must only be called for terminal, non-cancellation failures (i.e. a file we have
// genuinely given up on); cancellation and "not ready" outcomes use close, which records no
// file-level stat.
func (p *uploadProgressLogger) failure() {
	if p == nil {
		return
	}
	p.stop()
	p.stats.uploadFailedFileCount.Add(1)
}

// onResult records the outcome of a finished upload and stops the progress goroutine. It is
// the single call an upload path makes once retry.run returns, so callers no longer branch on
// the error to pick complete vs failure. bytesUploaded is ignored on the error path.
//
// Only a terminal, genuine failure increments uploadFailedFileCount. Outcomes that are not a
// permanent failure — cancellation (the file is retried next sync cycle) and the "not ready"
// conditions (empty or too-recently-modified, which resolve on their own) — are treated as a
// no-op close so the file-level failure counter stays a reliable signal. The per-attempt
// failure was already recorded by the caller via attemptFailed.
func (p *uploadProgressLogger) onResult(bytesUploaded uint64, err error) {
	if p == nil {
		return
	}
	switch {
	case err == nil:
		p.complete(bytesUploaded)
	case isTerminalFailure(err):
		p.failure()
	default:
		p.close()
	}
}

// isTerminalFailure reports whether err represents a permanent, give-up file failure that
// should be counted in uploadFailedFileCount. Cancellation and the "not ready" upload errors
// are excluded because the file will be retried (or resolves on its own) rather than failed.
func isTerminalFailure(err error) bool {
	return !errors.Is(err, context.Canceled) &&
		!errors.Is(err, errFileEmpty) &&
		!errors.Is(err, errFileModifiedTooRecently)
}

// close stops the progress goroutine without logging or recording stats. It is the deferred
// cleanup safety net: idempotent with complete and failure (a no-op if either already ran),
// so it guarantees the goroutine is stopped even on an early return or panic.
func (p *uploadProgressLogger) close() {
	if p == nil {
		return
	}
	p.stop()
}

func (p *uploadProgressLogger) stop() {
	p.stopOnce.Do(p.workers.Stop)
}

// percent renders sent/totalBytes as a percentage string, clamped at 100% (retries can
// push sent past totalBytes since re-sent chunks are re-counted).
func (p *uploadProgressLogger) percent(sent uint64) string {
	if p.totalBytes == 0 {
		return "?%"
	}
	if sent > p.totalBytes {
		sent = p.totalBytes
	}
	return fmt.Sprintf("%d%%", sent*100/p.totalBytes)
}

// rate returns bytes/sec over the given duration, or 0 for a non-positive duration.
func rate(bytes uint64, d time.Duration) uint64 {
	secs := d.Seconds()
	if secs <= 0 {
		return 0
	}
	return uint64(float64(bytes) / secs)
}
