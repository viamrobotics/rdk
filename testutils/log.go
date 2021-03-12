package testutils

import (
	"testing"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// NewLogger directs logs to the go test logger.
func NewLogger(t *testing.T) golog.Logger {
	logger, _ := NewObservedLogger(t)
	return logger
}

// NewObservedLogger is like NewLogger but also saves logs to an in memory observer.
func NewObservedLogger(t *testing.T) (golog.Logger, *observer.ObservedLogs) {
	logger := zaptest.NewLogger(t)
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	logger = logger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, observerCore)
	}))
	return logger.Sugar(), observedLogs
}
