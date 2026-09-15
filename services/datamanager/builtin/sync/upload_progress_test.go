package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/benbjohnson/clock"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/logging"
)

func TestUploadProgressLogger(t *testing.T) {
	t.Run("logs progress once per interval and a completion summary", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		var stats dataTypeUploadStats
		p := newUploadProgressLogger(logger, clk, "/tmp/big.bin", 100, &stats)

		// Half the file is sent, then an interval elapses, producing one progress log.
		p.addBytes(50)
		clk.Add(UploadProgressLogInterval)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 1)
		})
		info := observed.FilterLevelExact(zapcore.InfoLevel).All()[0]
		test.That(t, info.Message, test.ShouldContainSubstring, "uploading /tmp/big.bin")
		test.That(t, info.Message, test.ShouldContainSubstring, "50 Bytes / 100 Bytes")
		test.That(t, info.Message, test.ShouldContainSubstring, "(50%)")
		test.That(t, stats.uploadingBytes.Load(), test.ShouldEqual, 50)

		// Completion stops the progress goroutine and logs a single Debug summary reporting
		// the file size and average rate, and records the completed upload in the stats. The
		// summary is synchronous, so no wait is needed; the interval Info count stays at 1.
		p.addBytes(50)
		p.complete(100)
		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 1)
		summaries := observed.FilterLevelExact(zapcore.DebugLevel).All()
		test.That(t, len(summaries), test.ShouldEqual, 1)
		test.That(t, summaries[0].Message, test.ShouldContainSubstring, "Completed upload /tmp/big.bin")
		test.That(t, summaries[0].Message, test.ShouldContainSubstring, "100 Bytes")
		test.That(t, summaries[0].Message, test.ShouldContainSubstring, "avg rate:")
		test.That(t, stats.uploadingBytes.Load(), test.ShouldEqual, 100)
		test.That(t, stats.uploadedFileCount.Load(), test.ShouldEqual, 1)
		test.That(t, stats.completedUploadBytes.Load(), test.ShouldEqual, 100)
		test.That(t, stats.uploadFailedFileCount.Load(), test.ShouldEqual, 0)
	})

	t.Run("uploads that finish within one interval log only a completion summary", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		var stats dataTypeUploadStats
		p := newUploadProgressLogger(logger, clk, "/tmp/small.bin", 100, &stats)

		// No interval elapses before completion, so no in-flight Info progress log is emitted;
		// only the Debug completion summary.
		p.addBytes(100)
		p.complete(100)

		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 0)
		debugs := observed.FilterLevelExact(zapcore.DebugLevel).All()
		test.That(t, len(debugs), test.ShouldEqual, 1)
		test.That(t, debugs[0].Message, test.ShouldContainSubstring, "Completed upload /tmp/small.bin")
		test.That(t, stats.uploadedFileCount.Load(), test.ShouldEqual, 1)
	})

	t.Run("failure records a failed upload without logging a summary", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		var stats dataTypeUploadStats
		p := newUploadProgressLogger(logger, clk, "/tmp/failed.bin", 100, &stats)

		p.addBytes(30)
		p.failure()

		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 0)
		test.That(t, stats.uploadFailedFileCount.Load(), test.ShouldEqual, 1)
		test.That(t, stats.uploadedFileCount.Load(), test.ShouldEqual, 0)
		test.That(t, stats.completedUploadBytes.Load(), test.ShouldEqual, 0)
	})

	t.Run("close stops the goroutine without recording an outcome", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		var stats dataTypeUploadStats
		p := newUploadProgressLogger(logger, clk, "/tmp/cancelled.bin", 100, &stats)

		// close models a cancelled upload: no summary, no completed/failed counters touched.
		p.addBytes(30)
		p.close()

		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 0)
		test.That(t, stats.uploadedFileCount.Load(), test.ShouldEqual, 0)
		test.That(t, stats.uploadFailedFileCount.Load(), test.ShouldEqual, 0)
	})

	t.Run("onResult records success and terminal failure but skips cancellation", func(t *testing.T) {
		clk := clock.NewMock()

		var okStats dataTypeUploadStats
		ok := newUploadProgressLogger(logging.NewTestLogger(t), clk, "/tmp/ok.bin", 100, &okStats)
		ok.onResult(100, nil)
		test.That(t, okStats.uploadedFileCount.Load(), test.ShouldEqual, 1)
		test.That(t, okStats.uploadFailedFileCount.Load(), test.ShouldEqual, 0)

		// A genuine terminal error is counted as a file failure.
		var failStats dataTypeUploadStats
		bad := newUploadProgressLogger(logging.NewTestLogger(t), clk, "/tmp/bad.bin", 100, &failStats)
		bad.onResult(0, errors.New("boom"))
		test.That(t, failStats.uploadFailedFileCount.Load(), test.ShouldEqual, 1)
		test.That(t, failStats.uploadedFileCount.Load(), test.ShouldEqual, 0)

		// Cancellation is not a terminal failure: the file is retried next cycle, so no
		// file-level stat is recorded.
		var cancelStats dataTypeUploadStats
		cancelled := newUploadProgressLogger(logging.NewTestLogger(t), clk, "/tmp/cancel.bin", 100, &cancelStats)
		cancelled.onResult(0, context.Canceled)
		test.That(t, cancelStats.uploadFailedFileCount.Load(), test.ShouldEqual, 0)
		test.That(t, cancelStats.uploadedFileCount.Load(), test.ShouldEqual, 0)

		// The "not ready" upload errors are likewise not terminal failures.
		var emptyStats dataTypeUploadStats
		empty := newUploadProgressLogger(logging.NewTestLogger(t), clk, "/tmp/empty.bin", 0, &emptyStats)
		empty.onResult(0, errFileEmpty)
		test.That(t, emptyStats.uploadFailedFileCount.Load(), test.ShouldEqual, 0)
	})

	t.Run("percent reflects the current attempt after a retry re-sends", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		var stats dataTypeUploadStats
		p := newUploadProgressLogger(logger, clk, "/tmp/retry.bin", 100, &stats)

		// Attempt 1 sends 60 bytes then fails; attempt 2 resets and re-sends 50 bytes. Without
		// the per-attempt reset the display would read 110% (60+50); it must read 50%.
		p.startAttempt()
		p.addBytes(60)
		p.attemptFailed()
		p.startAttempt()
		p.addBytes(50)

		clk.Add(UploadProgressLogInterval)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 1)
		})
		msg := observed.FilterLevelExact(zapcore.InfoLevel).All()[0].Message
		test.That(t, msg, test.ShouldContainSubstring, "50 Bytes / 100 Bytes")
		test.That(t, msg, test.ShouldContainSubstring, "(50%)")
		test.That(t, msg, test.ShouldContainSubstring, "attempt 2")

		// The wire counter and FTDC in-progress bytes still reflect all 110 bytes actually sent.
		test.That(t, p.wireBytes.Load(), test.ShouldEqual, 110)
		test.That(t, stats.uploadingBytes.Load(), test.ShouldEqual, 110)

		// Attempt metrics are captured even though the file ultimately succeeds on retry.
		test.That(t, stats.uploadAttempts.Load(), test.ShouldEqual, 2)
		test.That(t, stats.uploadAttemptFailures.Load(), test.ShouldEqual, 1)

		// The second attempt succeeds: the file completes once, stopping the goroutine.
		p.complete(100)
		test.That(t, stats.uploadedFileCount.Load(), test.ShouldEqual, 1)
		test.That(t, stats.uploadFailedFileCount.Load(), test.ShouldEqual, 0)
	})
}
