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
	if imp.level.Get() == DEBUG {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		config.Level = GlobalLogLevel
	}
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

// Constructs the log message by forwarding to `fmt.Sprint`. `traceKey` may be the empty string.
func (imp *impl) format(logLevel Level, traceKey string, args ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprint(args...)
	if traceKey != emptyTraceKey {
		logEntry.fields = append(logEntry.fields, zap.String("traceKey", traceKey))
	}

	return logEntry
}

// Constructs the log message by forwarding to `fmt.Sprintf`. `traceKey` may be the empty string.
func (imp *impl) formatf(logLevel Level, traceKey, template string, args ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprintf(template, args...)
	if traceKey != emptyTraceKey {
		logEntry.fields = append(logEntry.fields, zap.String("traceKey", traceKey))
	}

	return logEntry
}

// Turns `keysAndValues` into a map where the odd elements are the keys and their following even
// counterpart is the value. The keys are expected to be strings. The values are json
// serialized. Only public fields are included in the serialization. `traceKey` may be the empty
// string.
func (imp *impl) formatw(logLevel Level, traceKey, msg string, keysAndValues ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = msg

	logEntry.fields = make([]zapcore.Field, 0, len(keysAndValues)/2+1)
	if traceKey != emptyTraceKey {
		logEntry.fields = append(logEntry.fields, zap.String("traceKey", traceKey))
	}

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
		imp.log(imp.format(DEBUG, emptyTraceKey, args...))
	}
}

func (imp *impl) CDebug(ctx context.Context, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		imp.log(imp.format(DEBUG, dbgName, args...))
	}
}

func (imp *impl) Debugf(template string, args ...interface{}) {
	if imp.shouldLog(DEBUG) {
		imp.log(imp.formatf(DEBUG, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CDebugf(ctx context.Context, template string, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		imp.log(imp.formatf(DEBUG, dbgName, template, args...))
	}
}

func (imp *impl) Debugw(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(DEBUG) {
		imp.log(imp.formatw(DEBUG, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CDebugw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		imp.log(imp.formatw(DEBUG, dbgName, msg, keysAndValues...))
	}
}

func (imp *impl) Info(args ...interface{}) {
	if imp.shouldLog(INFO) {
		imp.log(imp.format(INFO, emptyTraceKey, args...))
	}
}

func (imp *impl) CInfo(ctx context.Context, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for info, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		imp.log(imp.format(INFO, dbgName, args...))
	}
}

func (imp *impl) Infof(template string, args ...interface{}) {
	if imp.shouldLog(INFO) {
		imp.log(imp.formatf(INFO, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CInfof(ctx context.Context, template string, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for info, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		imp.log(imp.formatf(INFO, dbgName, template, args...))
	}
}

func (imp *impl) Infow(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(INFO) {
		imp.log(imp.formatw(INFO, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CInfow(ctx context.Context, msg string, keysAndValues ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for info, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		imp.log(imp.formatw(INFO, dbgName, msg, keysAndValues...))
	}
}

func (imp *impl) Warn(args ...interface{}) {
	if imp.shouldLog(WARN) {
		imp.log(imp.format(WARN, emptyTraceKey, args...))
	}
}

func (imp *impl) CWarn(ctx context.Context, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for warn, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		imp.log(imp.format(WARN, dbgName, args...))
	}
}

func (imp *impl) Warnf(template string, args ...interface{}) {
	if imp.shouldLog(WARN) {
		imp.log(imp.formatf(WARN, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CWarnf(ctx context.Context, template string, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for warn, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		imp.log(imp.formatf(WARN, dbgName, template, args...))
	}
}

func (imp *impl) Warnw(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(WARN) {
		imp.log(imp.formatw(WARN, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CWarnw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for warn, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		imp.log(imp.formatw(WARN, dbgName, msg, keysAndValues...))
	}
}

func (imp *impl) Error(args ...interface{}) {
	if imp.shouldLog(ERROR) {
		imp.log(imp.format(ERROR, emptyTraceKey, args...))
	}
}

func (imp *impl) CError(ctx context.Context, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for error, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		imp.log(imp.format(ERROR, dbgName, args...))
	}
}

func (imp *impl) Errorf(template string, args ...interface{}) {
	if imp.shouldLog(ERROR) {
		imp.log(imp.formatf(ERROR, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CErrorf(ctx context.Context, template string, args ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for error, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		imp.log(imp.formatf(ERROR, dbgName, template, args...))
	}
}

func (imp *impl) Errorw(msg string, keysAndValues ...interface{}) {
	if imp.shouldLog(ERROR) {
		imp.log(imp.formatw(ERROR, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CErrorw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	dbgName := GetName(ctx)

	// We log if the logger is configured for error, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		imp.log(imp.formatw(ERROR, dbgName, msg, keysAndValues...))
	}
}

// These Fatal* methods log as errors then exit the process.
func (imp *impl) Fatal(args ...interface{}) {
	imp.log(imp.format(ERROR, emptyTraceKey, args...))
	os.Exit(1)
}

func (imp *impl) Fatalf(template string, args ...interface{}) {
	imp.log(imp.formatf(ERROR, emptyTraceKey, template, args...))
	os.Exit(1)
}

func (imp *impl) Fatalw(msg string, keysAndValues ...interface{}) {
	imp.log(imp.formatw(ERROR, emptyTraceKey, msg, keysAndValues...))
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
