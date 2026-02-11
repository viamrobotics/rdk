package builtin

import (
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestCheckAndLogStaleData(t *testing.T) {
	t.Run("no warning when earliestTime is nil", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 0.1,
		}
		tracker.checkAndLogStaleData(nil, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)
	})

	t.Run("no warning when scheduler is disabled", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: false,
			syncIntervalMins: 0.1,
		}
		oldTime := time.Now().Add(-1 * time.Hour)
		tracker.checkAndLogStaleData(&oldTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)
	})

	t.Run("no warning when data is fresh", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 0.1,
		}
		recentTime := time.Now().Add(-30 * time.Second)
		tracker.checkAndLogStaleData(&recentTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)
	})

	t.Run("warning when data is stale and sync is enabled", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 0.1, // 6 seconds, threshold = max(60s, 3min) = 3min
		}
		staleTime := time.Now().Add(-5 * time.Minute)
		tracker.checkAndLogStaleData(&staleTime, 42, 5*1024*1024)
		warnLogs := logs.FilterMessageSnippet("Capture data may not be syncing")
		test.That(t, warnLogs.Len(), test.ShouldEqual, 1)
		test.That(t, warnLogs.All()[0].Message, test.ShouldContainSubstring, "42 files")
		test.That(t, warnLogs.All()[0].Message, test.ShouldContainSubstring, "5.0 MB")
	})

	t.Run("warning respects stale threshold based on sync interval", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		// With 2 min sync interval, threshold = 10 * 2 = 20 minutes.
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 2.0,
		}

		// 15 min old - under 20 min threshold, no warning.
		justUnder := time.Now().Add(-15 * time.Minute)
		tracker.checkAndLogStaleData(&justUnder, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)

		// 25 min old - over 20 min threshold, warning.
		over := time.Now().Add(-25 * time.Minute)
		tracker.checkAndLogStaleData(&over, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)
	})

	t.Run("warning is rate-limited", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 0.1,
		}
		staleTime := time.Now().Add(-5 * time.Minute)

		// First call should log.
		tracker.checkAndLogStaleData(&staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)

		// Second call immediately after should be rate-limited.
		tracker.checkAndLogStaleData(&staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)
	})

	t.Run("warning fires again after rate limit expires", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 0.1,
		}
		staleTime := time.Now().Add(-5 * time.Minute)

		// First call should log.
		tracker.checkAndLogStaleData(&staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)

		// Simulate that staleWarningInterval has passed.
		tracker.lastStaleWarning = time.Now().Add(-staleWarningInterval - time.Second)

		// Should log again.
		tracker.checkAndLogStaleData(&staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 2)
	})

	t.Run("minStaleThreshold applies for short sync intervals", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		// With 0.1 min (6s) sync interval, 10 * 0.1 = 1 min.
		// But minStaleThreshold is 3 min, so threshold should be 3 min.
		tracker := &diskSummaryTracker{
			logger:           logger,
			schedulerEnabled: true,
			syncIntervalMins: 0.1,
		}

		// 2 min old - over 1 min but under 3 min minimum, no warning.
		twoMinOld := time.Now().Add(-2 * time.Minute)
		tracker.checkAndLogStaleData(&twoMinOld, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)

		// 4 min old - over 3 min minimum, warning.
		fourMinOld := time.Now().Add(-4 * time.Minute)
		tracker.checkAndLogStaleData(&fourMinOld, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)
	})
}

func TestFormatBytes(t *testing.T) {
	test.That(t, formatBytes(500), test.ShouldEqual, "500 B")
	test.That(t, formatBytes(1024), test.ShouldEqual, "1.0 KB")
	test.That(t, formatBytes(1536), test.ShouldEqual, "1.5 KB")
	test.That(t, formatBytes(1048576), test.ShouldEqual, "1.0 MB")
	test.That(t, formatBytes(1073741824), test.ShouldEqual, "1.0 GB")
	test.That(t, formatBytes(1610612736), test.ShouldEqual, "1.5 GB")
}
