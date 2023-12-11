package logging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	impl struct {
		name  string
		level AtomicLevel
		inUTC bool

		appenders []Appender
	}

	// LogEntry embeds a zapcore Entry and slice of Fields.
	LogEntry struct {
		zapcore.Entry
		fields []zapcore.Field
	}
)

func (imp *impl) NewLogEntry() *LogEntry {
	ret := &LogEntry{}
	ret.Time = time.Now()
	ret.LoggerName = imp.name
	ret.Caller = getCaller()

	return ret
}

func (imp *impl) AddAppender(appender Appender) {
	imp.appenders = append(imp.appenders, appender)
}

func (imp *impl) Desugar() *zap.Logger {
	return imp.AsZap().Desugar()
}

func (imp *impl) SetLevel(level Level) {
	imp.level.Set(level)
}

func (imp *impl) GetLevel() Level {
	return imp.level.Get()
}

func (imp *impl) Level() zapcore.Level {
	return imp.GetLevel().AsZap()
}

func (imp *impl) Sublogger(subname string) Logger {
	newName := subname
	if imp.name != "" {
		newName = fmt.Sprintf("%s.%s", imp.name, subname)
	}

	return &impl{
		name:      newName,
		level:     NewAtomicLevelAt(imp.level.Get()),
		appenders: imp.appenders,
	}
}

func (imp *impl) Named(name string) *zap.SugaredLogger {
	return imp.AsZap().Named(name)
}

func (imp *impl) Sync() error {
	var errs []error
	for _, appender := range imp.appenders {
		if err := appender.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.Combine(errs...)
}

func (imp *impl) With(args ...interface{}) *zap.SugaredLogger {
	return imp.AsZap().With(args...)
}

func (imp *impl) WithOptions(opts ...zap.Option) *zap.SugaredLogger {
	return imp.AsZap().WithOptions(opts...)
}

func (imp *impl) AsZap() *zap.SugaredLogger {
	// When downconverting to a SugaredLogger, copy those that implement the `zapcore.Core`
	// interface. This includes the net logger for viam servers and the observed logs for tests.
	var copiedCores []zapcore.Core
	for _, appender := range imp.appenders {
		if core, ok := appender.(zapcore.Core); ok {
			copiedCores = append(copiedCores, core)
		}
	}

	config := NewZapLoggerConfig()
	// Use the global zap `AtomicLevel` such that the constructed zap logger can observe changes to
	// the debug flag.
	config.Level = GlobalLogLevel
	ret := zap.Must(config.Build()).Sugar().Named(imp.name)
	for _, core := range copiedCores {
		ret = ret.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewTee(c, core)
		}))
	}

	return ret
}

func (imp *impl) shouldLog(logLevel Level) bool {
	if GlobalLogLevel.Level() == zapcore.DebugLevel {
		return true
	}

	return logLevel >= imp.level.Get()
}

func (imp *impl) log(entry *LogEntry) {
	if imp.inUTC {
		entry.Time = entry.Time.UTC()
	}

	for _, appender := range imp.appenders {
		err := appender.Write(entry.Entry, entry.fields)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
		}
	}
}

// Constructs the log message by forwarding to `fmt.Sprint`.
func (imp *impl) format(logLevel Level, args ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprint(args...)

	return logEntry
}

// Constructs the log message by forwarding to `fmt.Sprintf`.
func (imp *impl) formatf(logLevel Level, template string, args ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprintf(template, args...)

	return logEntry
}

// Turns `keysAndValues` into a map where the odd elements are the keys and their following even
// counterpart is the value. The keys are expected to be strings. The values are json
// serialized. Only public fields are included in the serialization.
func (imp *impl) formatw(logLevel Level, msg string, keysAndValues ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = msg

	logEntry.fields = make([]zapcore.Field, 0, len(keysAndValues)/2)
	for keyIdx := 0; keyIdx < len(keysAndValues); keyIdx += 2 {
		keyObj := keysAndValues[keyIdx]
		var keyStr string
		if stringer, ok := keyObj.(fmt.Stringer); ok {
			keyStr = stringer.String()
		} else {
			keyStr = fmt.Sprintf("%v", keyObj)
		}

		if keyIdx+1 < len(keysAndValues) {
			logEntry.fields = append(logEntry.fields, zap.Any(keyStr, keysAndValues[keyIdx+1]))
		} else {
			// API mis-use. Rather than logging a logging mis-use, slip in an error message such
			// that we don't silenlty discard it.
			logEntry.fields = append(logEntry.fields, zap.Any(keyStr, errors.New("unpaired log key")))
		}
	}

	return logEntry
}

func (imp *impl) Debug(args ...interface{}) {
	if imp.shouldLog(DEBUG) {
		imp.log(imp.format(DEBUG, args...))
	}
}

func (imp *impl) CDebug(ctx context.Context, args ...interface{}) {
	if imp.shouldLog(DEBUG) || IsDebugMode(ctx) {
		imp.log(imp.format(DEBUG, args...))
	}
}

func (imp *impl) Debugf(template string, args ...interface{}) {
	if imp.shouldLog(DEBUG) {
		imp.log(imp.formatf(DEBUG, template, args...))
	}
}

func (imp *impl) CDebugf(ctx context.Context, template string, args ...interface{}) {
	if imp.shouldLog(DEBUG) || IsDebugMode(ctx) {
		imp.log(imp.formatf(DEBUG, template, args...))
	}
}

func (imp *impl) Debugw(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(DEBUG) {
		imp.log(imp.formatw(DEBUG, msg, keysAndValues...))
	}
}

func (imp *impl) CDebugw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(DEBUG) || IsDebugMode(ctx) {
		imp.log(imp.formatw(DEBUG, msg, keysAndValues...))
	}
}

func (imp *impl) Info(args ...interface{}) {
	if imp.shouldLog(INFO) {
		imp.log(imp.format(INFO, args...))
	}
}

func (imp *impl) Infof(template string, args ...interface{}) {
	if imp.shouldLog(INFO) {
		imp.log(imp.formatf(INFO, template, args...))
	}
}

func (imp *impl) Infow(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(INFO) {
		imp.log(imp.formatw(INFO, msg, keysAndValues...))
	}
}

func (imp *impl) Warn(args ...interface{}) {
	if imp.shouldLog(WARN) {
		imp.log(imp.format(WARN, args...))
	}
}

func (imp *impl) Warnf(template string, args ...interface{}) {
	if imp.shouldLog(WARN) {
		imp.log(imp.formatf(WARN, template, args...))
	}
}

func (imp *impl) Warnw(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(WARN) {
		imp.log(imp.formatw(WARN, msg, keysAndValues...))
	}
}

func (imp *impl) Error(args ...interface{}) {
	if imp.shouldLog(ERROR) {
		imp.log(imp.format(ERROR, args...))
	}
}

func (imp *impl) Errorf(template string, args ...interface{}) {
	if imp.shouldLog(ERROR) {
		imp.log(imp.formatf(ERROR, template, args...))
	}
}

func (imp *impl) Errorw(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(ERROR) {
		imp.log(imp.formatw(ERROR, msg, keysAndValues...))
	}
}

// These Fatal* methods log as errors then exit the process.
func (imp *impl) Fatal(args ...interface{}) {
	imp.log(imp.format(ERROR, args...))
	os.Exit(1)
}

func (imp *impl) Fatalf(template string, args ...interface{}) {
	imp.log(imp.formatf(ERROR, template, args...))
	os.Exit(1)
}

func (imp *impl) Fatalw(msg string, keysAndValues ...interface{}) {
	imp.log(imp.formatw(ERROR, msg, keysAndValues...))
	os.Exit(1)
}

// Return example: "logging/impl_test.go:36". `entryCaller` is an outParameter.
func getCaller() zapcore.EntryCaller {
	var ok bool
	var entryCaller zapcore.EntryCaller
	const skipToLogCaller = 4
	entryCaller.PC, entryCaller.File, entryCaller.Line, ok = runtime.Caller(skipToLogCaller)
	if !ok {
		return entryCaller
	}
	entryCaller.Defined = true

	// Getting an individual program counter and the file/line/function at that address can be
	// nuanced due to inlining. The alternative is getting all program counters on the stack and
	// iterating through the associated frames with `runtime.CallersFrames`. A note to future
	// readers that this choice is due to less coding/convenience.
	runtimeFunc := runtime.FuncForPC(entryCaller.PC)
	if runtimeFunc != nil {
		entryCaller.Function = runtimeFunc.Name()
	}

	return entryCaller
}
