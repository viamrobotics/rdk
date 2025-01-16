// Package logging package contains functionality for viam-server logging.
package logging

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/utils"
)

var (
	globalMu     sync.RWMutex
	globalLogger = NewDebugLogger("global")

	// GlobalLogLevel should be used whenever a zap logger is created that wants to obey the debug
	// flag from the CLI or robot config.
	GlobalLogLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
)

// ReplaceGlobal replaces the global loggers.
func ReplaceGlobal(logger Logger) {
	globalMu.Lock()
	globalLogger = logger
	globalMu.Unlock()
}

// Global returns the global logger.
func Global() Logger {
	return globalLogger
}

// NewZapLoggerConfig returns a new default logger config.
func NewZapLoggerConfig() zap.Config {
	// from https://github.com/uber-go/zap/blob/2314926ec34c23ee21f3dd4399438469668f8097/config.go#L135
	// but disable stacktraces, use same keys as prod, and color levels.
	return zap.Config{
		Level:    GlobalLogLevel,
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		DisableStacktrace: true,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}
}

// NewLogger returns a new logger that outputs Info+ logs to stdout in UTC.
func NewLogger(name string) Logger {
	logger := &impl{
		name:                     name,
		level:                    NewAtomicLevelAt(INFO),
		appenders:                []Appender{NewStdoutAppender()},
		registry:                 newRegistry(),
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	logger.registry.registerLogger(name, logger)
	return logger
}

// NewLoggerWithRegistry is the same as NewLogger but also returns the
// associated Registry.
func NewLoggerWithRegistry(name string) (Logger, *Registry) {
	reg := newRegistry()
	logger := &impl{
		name:                     name,
		level:                    NewAtomicLevelAt(INFO),
		appenders:                []Appender{NewStdoutAppender()},
		registry:                 reg,
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	logger.registry.registerLogger(name, logger)
	return logger, reg
}

// NewDebugLogger returns a new logger that outputs Debug+ logs to stdout in UTC.
func NewDebugLogger(name string) Logger {
	logger := &impl{
		name:                     name,
		level:                    NewAtomicLevelAt(DEBUG),
		appenders:                []Appender{NewStdoutAppender()},
		registry:                 newRegistry(),
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	logger.registry.registerLogger(name, logger)
	return logger
}

// NewBlankLogger returns a new logger that outputs Debug+ logs in UTC, but without any
// pre-existing appenders/outputs.
func NewBlankLogger(name string) Logger {
	logger := &impl{
		name:                     name,
		level:                    NewAtomicLevelAt(DEBUG),
		appenders:                []Appender{},
		registry:                 newRegistry(),
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	logger.registry.registerLogger(name, logger)
	return logger
}

// NewBlankLoggerWithRegistry returns a new logger that outputs Debug+ logs in UTC, but without any
// pre-existing appenders/outputs. It also returns the logger `Registry`.
func NewBlankLoggerWithRegistry(name string) (Logger, *Registry) {
	logger := &impl{
		name:                     name,
		level:                    NewAtomicLevelAt(DEBUG),
		appenders:                []Appender{},
		registry:                 newRegistry(),
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	logger.registry.registerLogger(name, logger)
	return logger, logger.registry
}

// NewTestLogger returns a new logger that outputs Debug+ logs to stdout in local time.
func NewTestLogger(tb testing.TB) Logger {
	logger, _ := NewObservedTestLogger(tb)
	return logger
}

// NewObservedTestLogger is like NewTestLogger but also saves logs to an in memory observer.
func NewObservedTestLogger(tb testing.TB) (Logger, *observer.ObservedLogs) {
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	logger := &impl{
		name:  tb.Name(),
		level: NewAtomicLevelAt(DEBUG),
		appenders: []Appender{
			NewTestAppender(tb),
			observerCore,
		},
		registry:                 newRegistry(),
		testHelper:               tb.Helper,
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	return logger, observedLogs
}

// NewObservedTestLoggerWithRegistry is like NewObservedTestLogger but also returns the
// associated registry. It also takes a name for the logger.
func NewObservedTestLoggerWithRegistry(tb testing.TB, name string) (Logger, *observer.ObservedLogs, *Registry) {
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	registry := newRegistry()
	logger := &impl{
		name:  name,
		level: NewAtomicLevelAt(DEBUG),
		appenders: []Appender{
			NewTestAppender(tb),
			observerCore,
		},
		registry:                 registry,
		testHelper:               tb.Helper,
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	return logger, observedLogs, registry
}

// MemLogger stores test logs in memory. And can write them on request with `OutputLogs`.
type MemLogger struct {
	Logger

	tb       testing.TB
	Observer *observer.ObservedLogs
}

// OutputLogs writes in-memory logs to the test object MemLogger was constructed with.
func (memLogger *MemLogger) OutputLogs() {
	appender := NewTestAppender(memLogger.tb)
	for _, loggedEntry := range memLogger.Observer.All() {
		utils.UncheckedError(appender.Write(loggedEntry.Entry, loggedEntry.Context))
	}
}

// NewInMemoryLogger creates a MemLogger that buffers test logs and only outputs them if
// requested or if the test fails. This is handy if a test is noisy, but the output is
// useful when the test fails.
func NewInMemoryLogger(tb testing.TB) *MemLogger {
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	logger := &impl{
		name:  "",
		level: NewAtomicLevelAt(DEBUG),
		appenders: []Appender{
			observerCore,
		},
		registry:                 newRegistry(),
		testHelper:               tb.Helper,
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}

	memLogger := &MemLogger{logger, tb, observedLogs}
	// Ensure that logs are always output on failure.
	tb.Cleanup(func() {
		if tb.Failed() {
			memLogger.OutputLogs()
		}
	})
	return memLogger
}
