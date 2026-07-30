package sync

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestUploadProgressLogger(t *testing.T) {
	t.Run("logs progress at Info once per interval and an Info summary on completion", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		p := newUploadProgressLogger(logger, clk, "/tmp/big.bin", 100)

		// Before the interval elapses, chunks do not produce progress logs.
		p.addBytes(10)
		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 0)

		// Once the interval has passed, the next chunk triggers a progress log.
		clk.Add(UploadProgressLogInterval)
		p.addBytes(40)
		infos := observed.FilterLevelExact(zapcore.InfoLevel).All()
		test.That(t, len(infos), test.ShouldEqual, 1)
		test.That(t, infos[0].Message, test.ShouldContainSubstring, "uploading /tmp/big.bin")
		test.That(t, infos[0].Message, test.ShouldContainSubstring, "50 Bytes / 100 Bytes")
		test.That(t, infos[0].Message, test.ShouldContainSubstring, "(50%)")

		// Within the same interval, further chunks do not log again.
		p.addBytes(25)
		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 1)

		// Completion logs at Info because a progress line was emitted.
		clk.Add(time.Second)
		p.addBytes(25)
		p.complete()
		infos = observed.FilterLevelExact(zapcore.InfoLevel).All()
		test.That(t, len(infos), test.ShouldEqual, 2)
		test.That(t, infos[1].Message, test.ShouldContainSubstring, "uploaded /tmp/big.bin")
		test.That(t, infos[1].Message, test.ShouldContainSubstring, "100 Bytes")
	})

	t.Run("uploads that finish within one interval stay quiet at Info", func(t *testing.T) {
		logger, observed := logging.NewObservedTestLogger(t)
		clk := clock.NewMock()
		p := newUploadProgressLogger(logger, clk, "/tmp/small.bin", 100)

		p.addBytes(100)
		p.complete()

		test.That(t, observed.FilterLevelExact(zapcore.InfoLevel).Len(), test.ShouldEqual, 0)
		// The completion summary still exists at Debug for debugging.
		debugs := observed.FilterLevelExact(zapcore.DebugLevel).All()
		test.That(t, len(debugs), test.ShouldEqual, 1)
		test.That(t, debugs[0].Message, test.ShouldContainSubstring, "uploaded /tmp/small.bin")
	})
}
