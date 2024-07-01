// Package logging package contains functionality for viam-server logging.
package logging

import (
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/utils"
)

var (
	globalMu     sync.RWMutex
	globalLogger = NewDebugLogger("startup")

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
	return &impl{
		name:       name,
		level:      NewAtomicLevelAt(INFO),
		appenders:  []Appender{NewStdoutAppender()},
		testHelper: func() {},
	}
}

// NewDebugLogger returns a new logger that outputs Debug+ logs to stdout in UTC.
func NewDebugLogger(name string) Logger {
	return &impl{
		name:       name,
		level:      NewAtomicLevelAt(DEBUG),
		appenders:  []Appender{NewStdoutAppender()},
		testHelper: func() {},
	}
}

// NewBlankLogger returns a new logger that outputs Debug+ logs in UTC, but without any
// pre-existing appenders/outputs.
func NewBlankLogger(name string) Logger {
	return &impl{
		name:       name,
		level:      NewAtomicLevelAt(DEBUG),
		appenders:  []Appender{},
		testHelper: func() {},
	}
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
		name:  "",
		level: NewAtomicLevelAt(DEBUG),
		appenders: []Appender{
			NewTestAppender(tb),
			observerCore,
		},
		testHelper: tb.Helper,
	}

	return logger, observedLogs
}

// MemLogger stores test logs in memory. And can write them on request with `OutputLogs`.
type MemLogger struct {
	Logger

	tb       testing.TB
	observer *observer.ObservedLogs
}

// OutputLogs writes in-memory logs to the test object MemLogger was constructed with.
func (memLogger *MemLogger) OutputLogs() {
	appender := NewTestAppender(memLogger.tb)
	for _, loggedEntry := range memLogger.observer.All() {
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
		testHelper: tb.Helper,
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
