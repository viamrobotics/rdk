package logging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var (
	// Window duration over which to consider log messages "noisy.".
	noisyMessageWindowDuration = 10 * time.Second
	// Count threshold within `noisyMessageWindowDuration` after which to
	// consider log messages "noisy.".
	noisyMessageCountThreshold = 3
)

type (
	impl struct {
		name  string
		level AtomicLevel

		appenders []Appender
		registry  *Registry
		// Logging to a `testing.T` always includes a filename/line number. We use this helper to
		// avoid that. This function is a no-op for non-test loggers. See `NewTestAppender`
		// documentation for more details.
		testHelper func()

		// recentMessageMu guards the recentMessage fields below.
		recentMessageMu sync.Mutex
		// Map of messages to counts of that message being `Write`ten within window.
		recentMessageCounts map[string]int
		// Map of messages to last `LogEntry` with that message within window.
		recentMessageEntries map[string]LogEntry
		// Start of current window.
		recentMessageWindowStart time.Time
	}

	// LogEntry embeds a zapcore Entry and slice of Fields.
	LogEntry struct {
		zapcore.Entry
		// Fields are the key-value fields of the entry.
		Fields []zapcore.Field
	}

	implWith struct {
		*impl
		logFields []zapcore.Field
	}
)

// HashKey creates a hash key string for a `LogEntry`. Should be used to emplace a log
// entry in `recentMessageEntries`, i.e. `LogEntry`s that `HashKey` identically should be
// treated as identical with respect to noisiness and deduplication.
func (le *LogEntry) HashKey() string {
	ret := le.Message
	for _, field := range le.Fields {
		ret += " " + field.Key + " "

		// Assume field's value is held in one of `Integer`, `Interface`, or
		// `String`. Otherwise (field has no value or is equivalent to 0 or "") use
		// the string "undefined".
		switch {
		case field.Integer != 0:
			ret += fmt.Sprintf("%d", field.Integer)
		case field.Interface != nil:
			ret += fmt.Sprintf("%v", field.Interface)
		case field.String != "":
			ret += field.String
		default:
			ret += "undefined"
		}
	}
	return ret
}

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

	// Force all parameters to be passed. Avoid bugs where adding members to `impl` silently
	// succeeds without a change here.
	sublogger := &impl{
		newName,
		NewAtomicLevelAt(imp.level.Get()),
		imp.appenders,
		imp.registry,
		imp.testHelper,
		sync.Mutex{},
		make(map[string]int),
		make(map[string]LogEntry),
		time.Now(),
	}

	// If there are multiple callers racing to create the same logger name (e.g: `viam.networking`),
	// all callers will create a `Sublogger`, but only one will "win" the race. All "losers" will
	// get the same instance that's guaranteed to be in the registry.
	//
	// `getOrRegister` will also set the appropriate log level based on the config.
	return imp.registry.getOrRegister(newName, sublogger)
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

func (imp *impl) WithOptions(opts ...zap.Option) *zap.SugaredLogger {
	return imp.AsZap().WithOptions(opts...)
}

func (imp *impl) WithFields(args ...interface{}) Logger {
	// fields will always get added as the last key-value pair(s) to a log line
	fields := make([]zapcore.Field, 0, len(args)/2+1)

	for keyIdx := 0; keyIdx < len(args); keyIdx += 2 {
		keyObj := args[keyIdx]
		var keyStr string

		switch key := keyObj.(type) {
		case string:
			keyStr = key
		case fmt.Stringer:
			keyStr = key.String()
		default:
			continue
		}

		if keyIdx+1 < len(args) {
			fields = append(fields, zap.Any(keyStr, args[keyIdx+1]))
		} else {
			// API mis-use. Rather than logging a logging mis-use, slip in an error message such
			// that we don't silenlty discard it.
			fields = append(fields, zap.Any(keyStr, errors.New("unpaired log key")))
		}
	}

	return &implWith{
		imp,
		fields,
	}
}

func (imp *impl) AsZap() *zap.SugaredLogger {
	// When downconverting to a SugaredLogger, copy those that implement the `zapcore.Core`
	// interface. This includes the net logger for viam servers and the observed logs for tests.
	var copiedCores []zapcore.Core

	// When we find a `testAppender`, copy the underlying `testing.TB` object and construct a
	// `zaptest.NewLogger` from it.
	var testingObj testing.TB
	for _, appender := range imp.appenders {
		if core, ok := appender.(zapcore.Core); ok {
			copiedCores = append(copiedCores, core)
		}
		if testAppender, ok := appender.(*testAppender); ok {
			testingObj = testAppender.tb
		}
	}

	var ret *zap.SugaredLogger
	if testingObj == nil {
		config := NewZapLoggerConfig()
		// Use the global zap `AtomicLevel` such that the constructed zap logger can observe changes to
		// the debug flag.
		if imp.level.Get() == DEBUG {
			config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		} else {
			config.Level = GlobalLogLevel
		}
		ret = zap.Must(config.Build()).Sugar().Named(imp.name)
	} else {
		ret = zaptest.NewLogger(testingObj,
			zaptest.WrapOptions(zap.AddCaller()),
			zaptest.Level(imp.level.Get().AsZap()),
		).Sugar().Named(imp.name)
	}

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

func (imp *impl) Write(entry *LogEntry) {
	if imp.registry.DeduplicateLogs.Load() {
		hashkeyedEntry := entry.HashKey()

		// If we have entered a new recentMessage window, output noisy logs from
		// the last window.
		imp.recentMessageMu.Lock()
		if time.Since(imp.recentMessageWindowStart) > noisyMessageWindowDuration {
			for stringifiedEntry, count := range imp.recentMessageCounts {
				if count > noisyMessageCountThreshold {
					collapsedEntry := imp.recentMessageEntries[stringifiedEntry]
					collapsedEntry.Message = fmt.Sprintf("Message logged %d times in past %v: %s",
						count, noisyMessageWindowDuration, collapsedEntry.Message)

					imp.testHelper()
					for _, appender := range imp.appenders {
						err := appender.Write(collapsedEntry.Entry, collapsedEntry.Fields)
						if err != nil {
							fmt.Fprint(os.Stderr, err)
						}
					}
				}
			}

			// Clear maps and reset window.
			clear(imp.recentMessageCounts)
			clear(imp.recentMessageEntries)
			imp.recentMessageWindowStart = time.Now()
		}

		// Track hashkeyed entry in recentMessage maps.
		imp.recentMessageCounts[hashkeyedEntry]++
		imp.recentMessageEntries[hashkeyedEntry] = *entry

		if imp.recentMessageCounts[hashkeyedEntry] > noisyMessageCountThreshold {
			// If entry's message is reportedly "noisy," return early.
			imp.recentMessageMu.Unlock()
			return
		}
		imp.recentMessageMu.Unlock()
	}

	imp.testHelper()
	for _, appender := range imp.appenders {
		err := appender.Write(entry.Entry, entry.Fields)
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
		logEntry.Fields = append(logEntry.Fields, zap.String("traceKey", traceKey))
	}

	return logEntry
}

// Constructs the log message by forwarding to `fmt.Sprintf`. `traceKey` may be the empty string.
func (imp *impl) formatf(logLevel Level, traceKey, template string, args ...interface{}) *LogEntry {
	logEntry := imp.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprintf(template, args...)

	if traceKey != emptyTraceKey {
		logEntry.Fields = append(logEntry.Fields, zap.String("traceKey", traceKey))
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

	logEntry.Fields = make([]zapcore.Field, 0, len(keysAndValues)/2+1)
	if traceKey != emptyTraceKey {
		logEntry.Fields = append(logEntry.Fields, zap.String("traceKey", traceKey))
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
			logEntry.Fields = append(logEntry.Fields, zap.Any(keyStr, keysAndValues[keyIdx+1]))
		} else {
			// API mis-use. Rather than logging a logging mis-use, slip in an error message such
			// that we don't silenlty discard it.
			logEntry.Fields = append(logEntry.Fields, zap.Any(keyStr, errors.New("unpaired log key")))
		}
	}

	return logEntry
}

func (imp *impl) Debug(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(DEBUG) {
		imp.Write(imp.format(DEBUG, emptyTraceKey, args...))
	}
}

func (imp *impl) CDebug(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		imp.Write(imp.format(DEBUG, dbgName, args...))
	}
}

func (imp *impl) Debugf(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(DEBUG) {
		imp.Write(imp.formatf(DEBUG, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CDebugf(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		imp.Write(imp.formatf(DEBUG, dbgName, template, args...))
	}
}

func (imp *impl) Debugw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(DEBUG) {
		imp.Write(imp.formatw(DEBUG, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CDebugw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		imp.Write(imp.formatw(DEBUG, dbgName, msg, keysAndValues...))
	}
}

func (imp *impl) Info(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(INFO) {
		imp.Write(imp.format(INFO, emptyTraceKey, args...))
	}
}

func (imp *impl) CInfo(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for info, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		imp.Write(imp.format(INFO, dbgName, args...))
	}
}

func (imp *impl) Infof(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(INFO) {
		imp.Write(imp.formatf(INFO, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CInfof(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for info, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		imp.Write(imp.formatf(INFO, dbgName, template, args...))
	}
}

func (imp *impl) Infow(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(INFO) {
		imp.Write(imp.formatw(INFO, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CInfow(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for info, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		imp.Write(imp.formatw(INFO, dbgName, msg, keysAndValues...))
	}
}

func (imp *impl) Warn(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(WARN) {
		imp.Write(imp.format(WARN, emptyTraceKey, args...))
	}
}

func (imp *impl) CWarn(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for warn, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		imp.Write(imp.format(WARN, dbgName, args...))
	}
}

func (imp *impl) Warnf(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(WARN) {
		imp.Write(imp.formatf(WARN, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CWarnf(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for warn, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		imp.Write(imp.formatf(WARN, dbgName, template, args...))
	}
}

func (imp *impl) Warnw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(WARN) {
		imp.Write(imp.formatw(WARN, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CWarnw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for warn, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		imp.Write(imp.formatw(WARN, dbgName, msg, keysAndValues...))
	}
}

func (imp *impl) Error(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(ERROR) {
		imp.Write(imp.format(ERROR, emptyTraceKey, args...))
	}
}

func (imp *impl) CError(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for error, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		imp.Write(imp.format(ERROR, dbgName, args...))
	}
}

func (imp *impl) Errorf(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(ERROR) {
		imp.Write(imp.formatf(ERROR, emptyTraceKey, template, args...))
	}
}

func (imp *impl) CErrorf(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for error, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		imp.Write(imp.formatf(ERROR, dbgName, template, args...))
	}
}

func (imp *impl) Errorw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(ERROR) {
		imp.Write(imp.formatw(ERROR, emptyTraceKey, msg, keysAndValues...))
	}
}

func (imp *impl) CErrorw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for error, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		imp.Write(imp.formatw(ERROR, dbgName, msg, keysAndValues...))
	}
}

// These Fatal* methods log as errors then exit the process.
func (imp *impl) Fatal(args ...interface{}) {
	imp.testHelper()
	imp.Write(imp.format(ERROR, emptyTraceKey, args...))
	os.Exit(1)
}

func (imp *impl) Fatalf(template string, args ...interface{}) {
	imp.testHelper()
	imp.Write(imp.formatf(ERROR, emptyTraceKey, template, args...))
	os.Exit(1)
}

func (imp *impl) Fatalw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	imp.Write(imp.formatw(ERROR, emptyTraceKey, msg, keysAndValues...))
	os.Exit(1)
}

// Return example: "logging/impl_test.go:36". `entryCaller` is an outParameter.
func getCaller() zapcore.EntryCaller {
	var ok bool
	var entryCaller zapcore.EntryCaller
	const framesToSkip = 4
	entryCaller.PC, entryCaller.File, entryCaller.Line, ok = runtime.Caller(framesToSkip)
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

func (imp *implWith) Debug(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(DEBUG) {
		entry := imp.format(DEBUG, emptyTraceKey, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CDebug(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		entry := imp.format(DEBUG, dbgName, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Debugf(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(DEBUG) {
		entry := imp.formatf(DEBUG, emptyTraceKey, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CDebugf(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		entry := imp.formatf(DEBUG, dbgName, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Debugw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(DEBUG) {
		entry := imp.formatw(DEBUG, emptyTraceKey, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CDebugw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(DEBUG) || dbgName != emptyTraceKey {
		entry := imp.formatw(DEBUG, dbgName, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Info(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(INFO) {
		entry := imp.format(INFO, emptyTraceKey, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CInfo(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		entry := imp.format(INFO, dbgName, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Infof(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(INFO) {
		entry := imp.formatf(INFO, emptyTraceKey, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CInfof(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		entry := imp.formatf(INFO, dbgName, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Infow(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(INFO) {
		entry := imp.formatw(INFO, emptyTraceKey, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CInfow(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(INFO) || dbgName != emptyTraceKey {
		entry := imp.formatw(INFO, dbgName, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Warn(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(WARN) {
		entry := imp.format(WARN, emptyTraceKey, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CWarn(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		entry := imp.format(WARN, dbgName, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Warnf(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(WARN) {
		entry := imp.formatf(WARN, emptyTraceKey, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CWarnf(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		entry := imp.formatf(WARN, dbgName, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Warnw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(WARN) {
		entry := imp.formatw(WARN, emptyTraceKey, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CWarnw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(WARN) || dbgName != emptyTraceKey {
		entry := imp.formatw(WARN, dbgName, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Error(args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(ERROR) {
		entry := imp.format(ERROR, emptyTraceKey, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CError(ctx context.Context, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		entry := imp.format(ERROR, dbgName, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Errorf(template string, args ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(ERROR) {
		entry := imp.formatf(ERROR, emptyTraceKey, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CErrorf(ctx context.Context, template string, args ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		entry := imp.formatf(ERROR, dbgName, template, args...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Errorw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	if imp.shouldLog(ERROR) {
		entry := imp.formatw(ERROR, emptyTraceKey, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) CErrorw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	dbgName := GetName(ctx)

	// We log if the logger is configured for debug, or if there's a trace key.
	if imp.shouldLog(ERROR) || dbgName != emptyTraceKey {
		entry := imp.formatw(ERROR, dbgName, msg, keysAndValues...)
		entry.Fields = append(entry.Fields, imp.logFields...)
		imp.Write(entry)
	}
}

func (imp *implWith) Fatal(args ...interface{}) {
	imp.testHelper()
	entry := imp.format(ERROR, emptyTraceKey, args...)
	entry.Fields = append(entry.Fields, imp.logFields...)
	imp.Write(entry)
	os.Exit(1)
}

func (imp *implWith) Fatalf(template string, args ...interface{}) {
	imp.testHelper()
	entry := imp.formatf(ERROR, emptyTraceKey, template, args...)
	entry.Fields = append(entry.Fields, imp.logFields...)
	imp.Write(entry)
	os.Exit(1)
}

func (imp *implWith) Fatalw(msg string, keysAndValues ...interface{}) {
	imp.testHelper()
	entry := imp.formatw(ERROR, emptyTraceKey, msg, keysAndValues...)
	entry.Fields = append(entry.Fields, imp.logFields...)
	imp.Write(entry)
	os.Exit(1)
}
