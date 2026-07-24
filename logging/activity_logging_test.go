package logging

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
)

// swapInObservedActivityLogger installs a fresh global activity logger backed by an
// observer core and restores the previous global on cleanup.
func swapInObservedActivityLogger(t *testing.T, unit string) (*Registry, *observer.ObservedLogs) {
	t.Helper()
	observerCore, logs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	old := globalActivityLogger
	t.Cleanup(func() { globalActivityLogger = old })
	registry := newRegistry()
	InitActivityLogger(registry, unit)
	globalActivityLogger.logger.AddAppender(observerCore)
	return registry, logs
}

func TestActivityEventFields(t *testing.T) {
	_, logs := swapInObservedActivityLogger(t, "testunit")

	Activity("reconfigure", "start", "revision", "rev123")

	all := logs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	entry := all[0]
	test.That(t, entry.LoggerName, test.ShouldEqual, "rdk.activity")
	test.That(t, entry.Level, test.ShouldEqual, zapcore.InfoLevel)
	test.That(t, entry.Message, test.ShouldEqual, "")

	fields := entry.ContextMap()
	test.That(t, fields["event_type"], test.ShouldEqual, "reconfigure")
	test.That(t, fields["event"], test.ShouldEqual, "start")
	test.That(t, fields["unit"], test.ShouldEqual, "testunit")
	test.That(t, fields["revision"], test.ShouldEqual, "rev123")
}

func TestActivityErrorLevel(t *testing.T) {
	_, logs := swapInObservedActivityLogger(t, "testunit")

	ActivityError("reconfigure", "fail", "errors", "boom")

	all := logs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	test.That(t, all[0].Level, test.ShouldEqual, zapcore.ErrorLevel)
	fields := all[0].ContextMap()
	test.That(t, fields["event_type"], test.ShouldEqual, "reconfigure")
	test.That(t, fields["event"], test.ShouldEqual, "fail")
	test.That(t, fields["unit"], test.ShouldEqual, "testunit")
	test.That(t, fields["errors"], test.ShouldEqual, "boom")
}

func TestActivityNeverDeduplicated(t *testing.T) {
	registry, logs := swapInObservedActivityLogger(t, "testunit")
	// Enable dedup at the registry level to prove activity events are exempt.
	registry.DeduplicateLogs.Store(true)

	n := noisyMessageCountThreshold + 3
	for range n {
		Activity("reconfigure", "start")
	}
	test.That(t, len(logs.All()), test.ShouldEqual, n)
}

func TestActivityCallerAttribution(t *testing.T) {
	_, logs := swapInObservedActivityLogger(t, "testunit")

	Activity("reconfigure", "start")

	all := logs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	caller := all[0].Caller
	test.That(t, caller.Defined, test.ShouldBeTrue)
	test.That(t, strings.Contains(caller.File, "activity_logging_test.go"), test.ShouldBeTrue)
}

func TestActivityDroppedWithoutSinks(t *testing.T) {
	// The package-default global has no appenders; Activity must be a safe no-op.
	old := globalActivityLogger
	t.Cleanup(func() { globalActivityLogger = old })
	InitActivityLogger(newRegistry(), "unknown")
	Activity("reconfigure", "start")
}
