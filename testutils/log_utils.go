package testutils

import (
	"testing"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// NewInfoObservedTestLogger is a copy of NewObservedTestLogger with info level
// debugging instead of debug level.
func NewInfoObservedTestLogger(tb testing.TB) (golog.Logger, *observer.ObservedLogs) {
	logger := zaptest.NewLogger(tb, zaptest.Level(zap.InfoLevel), zaptest.WrapOptions(zap.AddCaller()))
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.InfoLevel.Enabled))
	logger = logger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, observerCore)
	}))
	return logger.Sugar(), observedLogs
}
