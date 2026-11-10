package logging

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
)

func TestActivityEventFields(t *testing.T) {
	logger, _ := NewObservedTestLogger(t)
	activityLogs := NewObservedActivityLogger(t, logger)

	logger.Activity("reconfigure", "start", "revision", "rev123")

	all := activityLogs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	entry := all[0]
	test.That(t, entry.LoggerName, test.ShouldEqual, t.Name()+".activity")
	test.That(t, entry.Level, test.ShouldEqual, zapcore.InfoLevel)
	test.That(t, entry.Message, test.ShouldEqual, "")

	fields := entry.ContextMap()
	test.That(t, fields["activity"], test.ShouldEqual, "reconfigure")
	test.That(t, fields["event"], test.ShouldEqual, "start")
	test.That(t, fields["revision"], test.ShouldEqual, "rev123")
}

func TestActivityLoggerNameDerivation(t *testing.T) {
	// The activity logger is named for the root segment of the emitting logger's name,
	// so every logger in a tree shares one activity logger.
	logger, _ := NewLoggerWithRegistry("rdk")
	sub := logger.Sublogger("foo").Sublogger("bar")
	activityLogs := NewObservedActivityLogger(t, sub)

	sub.Activity("reconfigure", "start")

	all := activityLogs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	test.That(t, all[0].LoggerName, test.ShouldEqual, "rdk.activity")

	// The root logger resolves to the same activity logger.
	logger.Activity("reconfigure", "complete")
	test.That(t, len(activityLogs.All()), test.ShouldEqual, 2)
}

func TestActivityNeverDeduplicated(t *testing.T) {
	logger, _ := NewObservedTestLogger(t)
	// Enable dedup at the registry level to prove activity events are exempt.
	//nolint:forcetypeassert
	logger.(*impl).registry.DeduplicateLogs.Store(true)
	activityLogs := NewObservedActivityLogger(t, logger)

	n := noisyMessageCountThreshold + 3
	for range n {
		logger.Activity("reconfigure", "start")
	}
	test.That(t, len(activityLogs.All()), test.ShouldEqual, n)
}

func TestActivityIgnoresEmittingLoggerLevel(t *testing.T) {
	// Gating is on <root>.activity's level, not the emitting logger's, so raising a
	// component's level does not hide its lifecycle events.
	logger, _ := NewObservedTestLogger(t)
	logger.SetLevel(ERROR)
	activityLogs := NewObservedActivityLogger(t, logger)

	logger.Activity("reconfigure", "start")

	test.That(t, len(activityLogs.All()), test.ShouldEqual, 1)
}

func TestActivityRespectsActivityLoggerLevel(t *testing.T) {
	logger, _, registry := NewObservedTestLoggerWithRegistry(t, "rdk")
	activityLogs := NewObservedActivityLogger(t, logger)

	// Activity emits at INFO, so anything above it silences the feed.
	registry.Update([]LoggerPatternConfig{{Pattern: "rdk.activity", Level: "ERROR"}}, logger)
	logger.Activity("reconfigure", "start")
	test.That(t, activityLogs.Len(), test.ShouldEqual, 0)

	// Dropping the pattern returns the activity logger to the root's level.
	registry.Update(nil, logger)
	logger.Activity("reconfigure", "complete")
	test.That(t, activityLogs.Len(), test.ShouldEqual, 1)
}

func TestActivityLevelAppliesToLazilyCreatedLogger(t *testing.T) {
	// The activity logger is created on first use, so a pattern configured beforehand
	// has to reach it through getOrRegister rather than Update.
	logger, _, registry := NewObservedTestLoggerWithRegistry(t, "rdk")
	registry.Update([]LoggerPatternConfig{{Pattern: "rdk.activity", Level: "ERROR"}}, logger)

	activityLogs := NewObservedActivityLogger(t, logger)
	logger.Activity("reconfigure", "start")

	test.That(t, activityLogs.Len(), test.ShouldEqual, 0)
}

func TestActivityLevelFollowsWildcardPattern(t *testing.T) {
	// The activity logger is an ordinary registered logger, so broad patterns reach it.
	logger, _, registry := NewObservedTestLoggerWithRegistry(t, "rdk")
	activityLogs := NewObservedActivityLogger(t, logger)

	registry.Update([]LoggerPatternConfig{{Pattern: "rdk.*", Level: "ERROR"}}, logger)
	logger.Activity("reconfigure", "start")

	test.That(t, activityLogs.Len(), test.ShouldEqual, 0)
}

func TestActivityCallerAttribution(t *testing.T) {
	logger, _ := NewObservedTestLogger(t)
	activityLogs := NewObservedActivityLogger(t, logger)

	logger.Activity("reconfigure", "start")

	all := activityLogs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	caller := all[0].Caller
	test.That(t, caller.Defined, test.ShouldBeTrue)
	test.That(t, strings.Contains(caller.File, "activity_logging_test.go"), test.ShouldBeTrue)
}

func TestActivityLoggerInheritsAppenders(t *testing.T) {
	// A lazily created activity logger inherits the emitting logger's appenders, like a
	// Sublogger, so events reach the tree's sinks without any setup call.
	logger, registry := NewLoggerWithRegistry("rdk")
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	registry.AddAppenderToAll(observerCore)

	logger.Activity("reconfigure", "start")

	all := observedLogs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	test.That(t, all[0].LoggerName, test.ShouldEqual, "rdk.activity")
}

func TestActivityLoggerReceivesLaterAppenders(t *testing.T) {
	// Once created, the activity logger is registered, so appenders added to the whole
	// tree afterward reach it exactly once.
	logger, registry := NewLoggerWithRegistry("rdk")
	logger.Activity("reconfigure", "start")

	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	registry.AddAppenderToAll(observerCore)

	logger.Activity("reconfigure", "complete")
	test.That(t, len(observedLogs.All()), test.ShouldEqual, 1)
}

func TestActivityNoSinks(t *testing.T) {
	// An activity logger with no appenders must drop events without error. NewLogger
	// would supply a stdout appender for the activity logger to inherit, so this needs
	// the blank constructor to reach the empty-sink path.
	logger := NewBlankLogger("test")

	logger.Activity("reconfigure", "start")
}
