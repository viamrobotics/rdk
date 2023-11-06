// Package logging package contains functionality for viam-server logging.
package logging

import (
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var (
	globalMu     sync.RWMutex
	globalLogger = NewDebugLogger("startup")
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

// NewLoggerConfig returns a new default logger config.
func NewLoggerConfig() zap.Config {
	// from https://github.com/uber-go/zap/blob/2314926ec34c23ee21f3dd4399438469668f8097/config.go#L135
	// but disable stacktraces, use same keys as prod, and color levels.
	return zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.InfoLevel),
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
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
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
	const inUTC = true
	return &impl{name, NewAtomicLevelAt(INFO), inUTC, []Appender{NewStdoutAppender()}}
}

// NewDebugLogger returns a new logger that outputs Debug+ logs to stdout in UTC.
func NewDebugLogger(name string) Logger {
	const inUTC = true
	return &impl{name, NewAtomicLevelAt(DEBUG), inUTC, []Appender{NewStdoutAppender()}}
}

// NewBlankLogger returns a new logger that outputs Debug+ logs in UTC, but without any
// pre-existing appenders/outputs.
func NewBlankLogger(name string) Logger {
	const inUTC = true
	return &impl{name, NewAtomicLevelAt(DEBUG), inUTC, []Appender{}}
}

// NewTestLogger returns a new logger that outputs Debug+ logs to stdout in local time.
func NewTestLogger(tb testing.TB) Logger {
	logger, _ := NewObservedTestLogger(tb)
	return logger
}

// NewObservedTestLogger is like NewTestLogger but also saves logs to an in memory observer.
func NewObservedTestLogger(tb testing.TB) (Logger, *observer.ObservedLogs) {
	const inUTC = false
	logger := &impl{"", NewAtomicLevelAt(DEBUG), inUTC, []Appender{}}
	logger.AddAppender(NewStdoutTestAppender())

	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	logger.AddAppender(observerCore)

	return logger, observedLogs
}
