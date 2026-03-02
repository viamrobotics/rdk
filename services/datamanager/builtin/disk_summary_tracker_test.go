package builtin

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func alwaysSync(_ context.Context) bool  { return true }
func neverSync(_ context.Context) bool   { return false }

func TestCheckAndLogStaleData(t *testing.T) {
	ctx := context.Background()

	t.Run("no warning when earliestTime is nil", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 0.1,
		}
		tracker.checkAndLogStaleData(ctx, nil, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)
	})

	t.Run("no warning when shouldSync is nil", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			syncIntervalMins: 0.1,
		}
		oldTime := time.Now().Add(-1 * time.Hour)
		tracker.checkAndLogStaleData(ctx, &oldTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)
	})

	t.Run("no warning when data is fresh", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 0.1,
		}
		recentTime := time.Now().Add(-30 * time.Second)
		tracker.checkAndLogStaleData(ctx, &recentTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)
	})

	t.Run("WARN when data is stale and shouldSync returns true", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 0.1, // threshold = max(60s, 3min) = 3min
		}
		staleTime := time.Now().Add(-5 * time.Minute)
		tracker.checkAndLogStaleData(ctx, &staleTime, 42, 5*1024*1024)
		warnLogs := logs.FilterMessageSnippet("Capture data may not be syncing")
		test.That(t, warnLogs.Len(), test.ShouldEqual, 1)
		test.That(t, warnLogs.All()[0].Level.String(), test.ShouldEqual, "warn")
		test.That(t, warnLogs.All()[0].Message, test.ShouldContainSubstring, "42 files")
	})

	t.Run("DEBUG when data is stale and shouldSync returns false", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       neverSync,
			syncIntervalMins: 0.1,
		}
		staleTime := time.Now().Add(-5 * time.Minute)
		tracker.checkAndLogStaleData(ctx, &staleTime, 10, 1024)
		staleLogs := logs.FilterMessageSnippet("Capture data may not be syncing")
		test.That(t, staleLogs.Len(), test.ShouldEqual, 1)
		test.That(t, staleLogs.All()[0].Level.String(), test.ShouldEqual, "debug")
	})

	t.Run("warning respects stale threshold based on sync interval", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		// With 2 min sync interval, threshold = 10 * 2 = 20 minutes.
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 2.0,
		}

		// 15 min old - under 20 min threshold, no warning.
		justUnder := time.Now().Add(-15 * time.Minute)
		tracker.checkAndLogStaleData(ctx, &justUnder, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)

		// 25 min old - over 20 min threshold, warning.
		over := time.Now().Add(-25 * time.Minute)
		tracker.checkAndLogStaleData(ctx, &over, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)
	})

	t.Run("warning is rate-limited", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 0.1,
		}
		staleTime := time.Now().Add(-5 * time.Minute)

		// First call should log.
		tracker.checkAndLogStaleData(ctx, &staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)

		// Second call immediately after should be rate-limited.
		tracker.checkAndLogStaleData(ctx, &staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)
	})

	t.Run("warning fires again after rate limit expires", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 0.1,
		}
		staleTime := time.Now().Add(-5 * time.Minute)

		// First call should log.
		tracker.checkAndLogStaleData(ctx, &staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)

		// Simulate that staleWarningInterval has passed.
		tracker.lastStaleWarning = time.Now().Add(-staleWarningInterval - time.Second)

		// Should log again.
		tracker.checkAndLogStaleData(ctx, &staleTime, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 2)
	})

	t.Run("minStaleThreshold applies for short sync intervals", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		// With 0.1 min (6s) sync interval, 10 * 0.1 = 1 min.
		// But minStaleThreshold is 3 min, so threshold should be 3 min.
		tracker := &diskSummaryTracker{
			logger:           logger,
			shouldSync:       alwaysSync,
			syncIntervalMins: 0.1,
		}

		// 2 min old - over 1 min but under 3 min minimum, no warning.
		twoMinOld := time.Now().Add(-2 * time.Minute)
		tracker.checkAndLogStaleData(ctx, &twoMinOld, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 0)

		// 4 min old - over 3 min minimum, warning.
		fourMinOld := time.Now().Add(-4 * time.Minute)
		tracker.checkAndLogStaleData(ctx, &fourMinOld, 10, 1024)
		test.That(t, logs.FilterMessageSnippet("Capture data may not be syncing").Len(), test.ShouldEqual, 1)
	})
}
