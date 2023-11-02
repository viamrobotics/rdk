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

// NewLogger returns a new logger using the default production configuration.
func NewLogger(name string) Logger {
	config := NewLoggerConfig()
	return &zLogger{zap.Must(config.Build()).Sugar().Named(name)}
}

// NewDebugLogger returns a new logger using the default debug configuration.
func NewDebugLogger(name string) Logger {
	config := NewLoggerConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	return &zLogger{zap.Must(config.Build()).Sugar().Named(name)}
}

// NewTestLogger directs logs to the go test logger.
func NewTestLogger(tb testing.TB) Logger {
	logger, _ := NewObservedTestLogger(tb)
	return logger
}

// NewObservedTestLogger is like NewTestLogger but also saves logs to an in memory observer.
func NewObservedTestLogger(tb testing.TB) (Logger, *observer.ObservedLogs) {
	logger := NewViamLogger("")
	logger.AddAppender(NewStdoutAppender())
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	logger.AddAppender(observerCore)

	return logger, observedLogs
}

// NewViamLogger creates an instance of the viam logger in debug mode without any outputs.
func NewViamLogger(name string) Logger {
	return &impl{name, DEBUG, []Appender{}}
}
