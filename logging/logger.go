package logging

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// Logger interface for logging to.
type Logger interface {
	Desugar() *zap.Logger
	Level() zapcore.Level
	Named(name string) *zap.SugaredLogger
	Sync() error
	With(args ...interface{}) *zap.SugaredLogger
	WithOptions(opts ...zap.Option) *zap.SugaredLogger

	AsZap() *zap.SugaredLogger

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

// ZLogger type for logging to. Wraps a zap logger and adds the `AsZap` method to satisfy the
// `Logger` interface.
type ZLogger struct {
	*zap.SugaredLogger
}

// AsZap converts the logger to a zap logger.
func (logger ZLogger) AsZap() *zap.SugaredLogger {
	return logger.SugaredLogger
}

var (
	globalMu     sync.RWMutex
	globalLogger = newDefaultLogger()
)

func newDefaultLogger() Logger {
	return ZLogger{zap.Must(NewDebugLoggerConfig().Build()).Sugar()}
}

// ReplaceGloabl replaces the global loggers and returns a function to reset
// the loggers to the previous state.
func ReplaceGloabl(logger Logger) func() {
	globalMu.Lock()
	prevLogger := globalLogger
	globalLogger = logger
	globalMu.Unlock()

	return func() {
		ReplaceGloabl(prevLogger)
	}
}

// Global returns the global logger.
func Global() Logger {
	globalMu.RLock()
	s := globalLogger
	globalMu.RUnlock()

	return s
}

// NewProductionLoggerConfig returns a new default production configuration.
func NewProductionLoggerConfig() zap.Config {
	// from https://github.com/uber-go/zap/blob/2314926ec34c23ee21f3dd4399438469668f8097/config.go#L98
	// but disable stacktraces.
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.EpochTimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		DisableStacktrace: true,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}
}

// NewLoggerConfigForGCP returns a new default production configuration for GCP.
func NewLoggerConfigForGCP() zap.Config {
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "severity",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    encodeLevel,
			EncodeTime:     rFC3339NanoTimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

// NewDevelopmentLoggerConfig returns a new default development logger config.
func NewDevelopmentLoggerConfig() zap.Config {
	// from https://github.com/uber-go/zap/blob/2314926ec34c23ee21f3dd4399438469668f8097/config.go#L135
	// but disable stacktraces, use same keys as prod, and color levels.
	logger := NewDebugLoggerConfig()
	logger.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	return logger
}

// NewDebugLoggerConfig returns a new default development logger config.
func NewDebugLoggerConfig() zap.Config {
	// from https://github.com/uber-go/zap/blob/2314926ec34c23ee21f3dd4399438469668f8097/config.go#L135
	// but disable stacktraces, use same keys as prod, and color levels.
	return zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.DebugLevel),
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
	logger, err := NewProductionLoggerConfig().Build()
	if err != nil {
		Global().Fatal(err)
	}
	return &ZLogger{logger.Sugar().Named(name)}
}

// NewLoggerForGCP returns a new logger using the default production configuration.
func NewLoggerForGCP(name string) Logger {
	logger, err := NewLoggerConfigForGCP().Build()
	if err != nil {
		Global().Fatal(err)
	}
	return &ZLogger{logger.Sugar().Named(name)}
}

// NewDevelopmentLogger returns a new logger using the default development configuration.
func NewDevelopmentLogger(name string) Logger {
	logger, err := NewDevelopmentLoggerConfig().Build()
	if err != nil {
		Global().Fatal(err)
	}
	return &ZLogger{logger.Sugar().Named(name)}
}

// NewDebugLogger returns a new logger using the default debug configuration.
func NewDebugLogger(name string) Logger {
	logger, err := NewDebugLoggerConfig().Build()
	if err != nil {
		Global().Fatal(err)
	}
	return &ZLogger{logger.Sugar().Named(name)}
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
	return &ZLogger{logger.Sugar()}, observedLogs
}

// rFC3339NanoTimeEncoder serializes a time.Time to an RFC3339Nano-formatted string with nanoseconds precision.
func rFC3339NanoTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(time.RFC3339Nano))
}

// encodeLevel maps the internal Zap log level to the appropriate Stackdriver level.
func encodeLevel(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(logLevelSeverity[l])
}

// See: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
var logLevelSeverity = map[zapcore.Level]string{
	zapcore.DebugLevel:  "DEBUG",
	zapcore.InfoLevel:   "INFO",
	zapcore.WarnLevel:   "WARNING",
	zapcore.ErrorLevel:  "ERROR",
	zapcore.DPanicLevel: "CRITICAL",
	zapcore.PanicLevel:  "ALERT",
	zapcore.FatalLevel:  "EMERGENCY",
}
