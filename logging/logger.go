package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger interface for logging to.
type Logger interface {
	ZapCompatibleLogger

	AsZap() *zap.SugaredLogger
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
	Debugln(args ...interface{})
	Debugw(msg string, keysAndValues ...interface{})

	Info(args ...interface{})
	Infof(template string, args ...interface{})
	Infoln(args ...interface{})
	Infow(msg string, keysAndValues ...interface{})

	Warn(args ...interface{})
	Warnf(template string, args ...interface{})
	Warnln(args ...interface{})
	Warnw(msg string, keysAndValues ...interface{})

	Error(args ...interface{})
	Errorf(template string, args ...interface{})
	Errorln(args ...interface{})
	Errorw(msg string, keysAndValues ...interface{})

	Fatal(args ...interface{})
	Fatalf(template string, args ...interface{})
	Fatalln(args ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})

	Panic(args ...interface{})
	Panicf(template string, args ...interface{})
	Panicln(args ...interface{})
	Panicw(msg string, keysAndValues ...interface{})

	DPanic(args ...interface{})
	DPanicf(template string, args ...interface{})
	DPanicln(args ...interface{})
	DPanicw(msg string, keysAndValues ...interface{})
}

// zLogger type for logging to. Wraps a zap logger and adds the `AsZap` method to satisfy the
// `Logger` interface.
type zLogger struct {
	*zap.SugaredLogger
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

// AsZap converts the logger to a zap logger.
func (logger *zLogger) AsZap() *zap.SugaredLogger {
	return logger.SugaredLogger
}

var _ Logger = &zLogger{}
