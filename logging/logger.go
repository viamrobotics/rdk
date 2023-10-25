package logging

import (
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
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

var (
	globalMu     sync.RWMutex
	globalLogger = NewDebugLogger("startup")
)

// ReplaceGloabl replaces the global loggers and returns a function to reset
// the loggers to the previous state.
func ReplaceGlobal(logger Logger) {
	globalMu.Lock()
	globalLogger = logger
	globalMu.Unlock()
}

// Global returns the global logger.
func Global() Logger {
	return globalLogger
}

// NewDebugLoggerConfig returns a new default development logger config.
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
	logger := zaptest.NewLogger(tb, zaptest.WrapOptions(zap.AddCaller()))
	observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.DebugLevel.Enabled))
	logger = logger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, observerCore)
	}))
	return &zLogger{logger.Sugar()}, observedLogs
}
