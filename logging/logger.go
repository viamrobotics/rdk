package logging

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger interface for logging to.
type Logger interface {
	ZapCompatibleLogger

	SetLevel(level Level)
	GetLevel() Level
	Sublogger(subname string) Logger
	AddAppender(appender Appender)
	AsZap() *zap.SugaredLogger

	CDebug(ctx context.Context, args ...interface{})
	CDebugf(ctx context.Context, template string, args ...interface{})
	CDebugw(ctx context.Context, msg string, keysAndValues ...interface{})

	CInfo(ctx context.Context, args ...interface{})
	CInfof(ctx context.Context, template string, args ...interface{})
	CInfow(ctx context.Context, msg string, keysAndValues ...interface{})

	CWarn(ctx context.Context, args ...interface{})
	CWarnf(ctx context.Context, template string, args ...interface{})
	CWarnw(ctx context.Context, msg string, keysAndValues ...interface{})

	CError(ctx context.Context, args ...interface{})
	CErrorf(ctx context.Context, template string, args ...interface{})
	CErrorw(ctx context.Context, msg string, keysAndValues ...interface{})
}

// ZapCompatibleLogger is a backwards compatibility layer for existing usages of the RDK as a
// library for Go application code or modules. Public (to the library) methods that take a logger as
// input should accept this type and upconvert to a Logger via a call to `FromZapCompatible`.
type ZapCompatibleLogger interface {
	Desugar() *zap.Logger
	Level() zapcore.Level
	Named(name string) *zap.SugaredLogger
	Sync() error
	With(args ...interface{}) *zap.SugaredLogger
	WithOptions(opts ...zap.Option) *zap.SugaredLogger

	Debug(args ...interface{})
	Debugf(template string, args ...interface{})
	Debugw(msg string, keysAndValues ...interface{})

	Info(args ...interface{})
	Infof(template string, args ...interface{})
	Infow(msg string, keysAndValues ...interface{})

	Warn(args ...interface{})
	Warnf(template string, args ...interface{})
	Warnw(msg string, keysAndValues ...interface{})

	Error(args ...interface{})
	Errorf(template string, args ...interface{})
	Errorw(msg string, keysAndValues ...interface{})

	Fatal(args ...interface{})
	Fatalf(template string, args ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
}

// FromZapCompatible upconverts a ZapCompatibleLogger to a logging.Logger. If the argument already
// satisfies logging.Logger, no changes will be made. A nil input returns a nil logger. An input of
// unknown type will create a new logger that's not associated with the input.
func FromZapCompatible(logger ZapCompatibleLogger) Logger {
	if logger == nil {
		return nil
	}

	switch l := logger.(type) {
	case *zap.SugaredLogger:
		// golog.Logger is a type alias for *zap.SugaredLogger and is captured by this.
		return &zLogger{l}
	case Logger:
		return l
	default:
		logger.Warnf("Unknown logger type, creating a new Viam Logger. Unknown type: %T", logger)
		return NewLogger("")
	}
}

// zLogger type for logging to. Wraps a zap logger and adds the `AsZap` method to satisfy the
// `Logger` interface.
type zLogger struct {
	*zap.SugaredLogger
}

var _ Logger = &zLogger{}

func (logger *zLogger) SetLevel(level Level) {
	// Not supported
}

func (logger *zLogger) GetLevel() Level {
	// Not supported
	return INFO
}

func (logger *zLogger) AddAppender(appender Appender) {
	// Not supported
}

// AsZap converts the logger to a zap logger.
func (logger *zLogger) AsZap() *zap.SugaredLogger {
	return logger.SugaredLogger
}

func (logger zLogger) Sublogger(name string) Logger {
	return &zLogger{logger.AsZap().Named(name)}
}

func (logger zLogger) CDebug(ctx context.Context, args ...interface{}) {
	logger.Debug(args...)
}

func (logger zLogger) CDebugf(ctx context.Context, template string, args ...interface{}) {
	logger.Debugf(template, args...)
}

func (logger zLogger) CDebugw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	logger.Debugw(msg, keysAndValues...)
}

func (logger zLogger) CInfo(ctx context.Context, args ...interface{}) {
	logger.Info(args...)
}

func (logger zLogger) CInfof(ctx context.Context, template string, args ...interface{}) {
	logger.Infof(template, args...)
}

func (logger zLogger) CInfow(ctx context.Context, msg string, keysAndValues ...interface{}) {
	logger.Infow(msg, keysAndValues...)
}

func (logger zLogger) CWarn(ctx context.Context, args ...interface{}) {
	logger.Warn(args...)
}

func (logger zLogger) CWarnf(ctx context.Context, template string, args ...interface{}) {
	logger.Warnf(template, args...)
}

func (logger zLogger) CWarnw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	logger.Warnw(msg, keysAndValues...)
}

func (logger zLogger) CError(ctx context.Context, args ...interface{}) {
	logger.Error(args...)
}

func (logger zLogger) CErrorf(ctx context.Context, template string, args ...interface{}) {
	logger.Errorf(template, args...)
}

func (logger zLogger) CErrorw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	logger.Errorw(msg, keysAndValues...)
}
