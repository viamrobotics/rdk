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

	CDebug(ctx context.Context, args ...any)
	CDebugf(ctx context.Context, template string, args ...any)
	CDebugw(ctx context.Context, msg string, keysAndValues ...any)

	CInfo(ctx context.Context, args ...any)
	CInfof(ctx context.Context, template string, args ...any)
	CInfow(ctx context.Context, msg string, keysAndValues ...any)

	CWarn(ctx context.Context, args ...any)
	CWarnf(ctx context.Context, template string, args ...any)
	CWarnw(ctx context.Context, msg string, keysAndValues ...any)

	CError(ctx context.Context, args ...any)
	CErrorf(ctx context.Context, template string, args ...any)
	CErrorw(ctx context.Context, msg string, keysAndValues ...any)
}

// ZapCompatibleLogger is a backwards compatibility layer for existing usages of the RDK as a
// library for Go application code or modules. Public (to the library) methods that take a logger as
// input should accept this type and upconvert to a Logger via a call to `FromZapCompatible`.
type ZapCompatibleLogger interface {
	Desugar() *zap.Logger
	Level() zapcore.Level
	Named(name string) *zap.SugaredLogger
	Sync() error
	With(args ...any) *zap.SugaredLogger
	WithOptions(opts ...zap.Option) *zap.SugaredLogger

	Debug(args ...any)
	Debugf(template string, args ...any)
	Debugw(msg string, keysAndValues ...any)

	Info(args ...any)
	Infof(template string, args ...any)
	Infow(msg string, keysAndValues ...any)

	Warn(args ...any)
	Warnf(template string, args ...any)
	Warnw(msg string, keysAndValues ...any)

	Error(args ...any)
	Errorf(template string, args ...any)
	Errorw(msg string, keysAndValues ...any)

	Fatal(args ...any)
	Fatalf(template string, args ...any)
	Fatalw(msg string, keysAndValues ...any)
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

func (logger zLogger) CDebug(ctx context.Context, args ...any) {
	logger.Debug(args...)
}

func (logger zLogger) CDebugf(ctx context.Context, template string, args ...any) {
	logger.Debugf(template, args...)
}

func (logger zLogger) CDebugw(ctx context.Context, msg string, keysAndValues ...any) {
	logger.Debugw(msg, keysAndValues...)
}

func (logger zLogger) CInfo(ctx context.Context, args ...any) {
	logger.Info(args...)
}

func (logger zLogger) CInfof(ctx context.Context, template string, args ...any) {
	logger.Infof(template, args...)
}

func (logger zLogger) CInfow(ctx context.Context, msg string, keysAndValues ...any) {
	logger.Infow(msg, keysAndValues...)
}

func (logger zLogger) CWarn(ctx context.Context, args ...any) {
	logger.Warn(args...)
}

func (logger zLogger) CWarnf(ctx context.Context, template string, args ...any) {
	logger.Warnf(template, args...)
}

func (logger zLogger) CWarnw(ctx context.Context, msg string, keysAndValues ...any) {
	logger.Warnw(msg, keysAndValues...)
}

func (logger zLogger) CError(ctx context.Context, args ...any) {
	logger.Error(args...)
}

func (logger zLogger) CErrorf(ctx context.Context, template string, args ...any) {
	logger.Errorf(template, args...)
}

func (logger zLogger) CErrorw(ctx context.Context, msg string, keysAndValues ...any) {
	logger.Errorw(msg, keysAndValues...)
}
