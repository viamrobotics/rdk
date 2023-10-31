package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	impl struct {
		name  string
		level Level

		out io.Writer
	}

	// LogEntry embeds a zapcore Entry and slice of Fields.
	LogEntry struct {
		zapcore.Entry
		fields []zapcore.Field
	}
)

func (impl impl) NewLogEntry() *LogEntry {
	ret := &LogEntry{}
	ret.Time = time.Now()
	ret.LoggerName = impl.name
	ret.Caller = getCaller()

	return ret
}

// The input `caller` must satisfy `caller.Defined == true`.
func callerToString(caller *zapcore.EntryCaller) string {
	// The file returned by `runtime.Caller` is a full path and always contains '/' to separate
	// directories. Including on windows. We only want to keep the `<package>/<file>` part of the
	// path. We use a stateful lambda to count back two '/' runes.
	cnt := 0
	idx := strings.LastIndexFunc(caller.File, func(rn rune) bool {
		if rn == '/' {
			cnt++
		}

		if cnt == 2 {
			return true
		}

		return false
	})

	// If idx >= 0, then we add 1 to trim the leading '/'.
	// If idx == -1 (not found), we add 1 to return the entire file.
	return fmt.Sprintf("%s:%d", caller.File[idx+1:], caller.Line)
}

// ToConsoleString turns a LogEntry into a string suitable for printing to stdout or writing to a file.
func (logEntry *LogEntry) ToConsoleString() string {
	const maxLength = 10
	toPrint := make([]string, 0, maxLength)
	toPrint = append(toPrint, logEntry.Time.Format("2006-01-02T15:04:05.000Z"))
	toPrint = append(toPrint, strings.ToUpper(logEntry.Level.String()))
	if logEntry.Caller.Defined {
		toPrint = append(toPrint, callerToString(&logEntry.Caller))
	}
	toPrint = append(toPrint, logEntry.Message)
	if len(logEntry.fields) == 0 {
		return strings.Join(toPrint, "\t")
	}

	jsonEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{SkipLineEnding: true})
	buf, err := jsonEncoder.EncodeEntry(zapcore.Entry{}, logEntry.fields)
	if err != nil {
		panic(err)
	}
	toPrint = append(toPrint, string(buf.Bytes()))

	return strings.Join(toPrint, "\t")
}

func (impl *impl) Desugar() *zap.Logger {
	return nil
}

func (impl *impl) Level() zapcore.Level {
	return zapcore.InfoLevel
}

func (impl *impl) Named(name string) *zap.SugaredLogger {
	return nil
}

func (impl *impl) Sync() error {
	return nil
}

func (impl *impl) With(args ...interface{}) *zap.SugaredLogger {
	return nil
}

func (impl *impl) WithOptions(opts ...zap.Option) *zap.SugaredLogger {
	return nil
}

func (impl *impl) shouldLog(logLevel Level) bool {
	return logLevel >= impl.level
}

func (impl *impl) log(entry *LogEntry) {
	fmt.Fprintln(impl.out, entry.ToConsoleString())
}

// Constructs the log message by forwarding to `fmt.Sprint`.
func (impl *impl) format(logLevel Level, args ...interface{}) *LogEntry {
	logEntry := impl.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprint(args...)

	return logEntry
}

// Constructs the log message by forwarding to `fmt.Sprintf`.
func (impl *impl) formatf(logLevel Level, template string, args ...interface{}) *LogEntry {
	logEntry := impl.NewLogEntry()
	logEntry.Level = logLevel.AsZap()
	logEntry.Message = fmt.Sprintf(template, args...)

	return logEntry
}

// Turns `keysAndValues` into a map where the odd elements are the keys and their following even
// counterpart is the value. The keys are expected to be strings. The values are json
// serialized. Only public fields are included in the serialization.
func (impl *impl) formatw(logLevel Level, msg string, keysAndValues ...interface{}) *LogEntry {
	logEntry := impl.NewLogEntry()
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
			// API mis-use
			logEntry.fields = append(logEntry.fields, zap.Any(keyStr, errors.New("unpaired log key")))
		}
	}

	return logEntry
}

func (impl *impl) Debug(args ...interface{}) {
	if impl.shouldLog(DEBUG) {
		impl.log(impl.format(DEBUG, args...))
	}
}

func (impl *impl) Debugf(template string, args ...interface{}) {
	if impl.shouldLog(DEBUG) {
		impl.log(impl.formatf(DEBUG, template, args...))
	}
}

func (impl *impl) Debugw(msg string, keysAndValues ...interface{}) {
	if impl.shouldLog(DEBUG) {
		impl.log(impl.formatw(DEBUG, msg, keysAndValues...))
	}
}

func (impl *impl) Info(args ...interface{}) {
	if impl.shouldLog(INFO) {
		impl.log(impl.format(INFO, args...))
	}
}

func (impl *impl) Infof(template string, args ...interface{}) {
	if impl.shouldLog(INFO) {
		impl.log(impl.formatf(INFO, template, args...))
	}
}

func (impl *impl) Infow(msg string, keysAndValues ...interface{}) {
	if impl.shouldLog(INFO) {
		impl.log(impl.formatw(INFO, msg, keysAndValues...))
	}
}

func (impl *impl) Warn(args ...interface{}) {
	if impl.shouldLog(WARN) {
		impl.log(impl.format(WARN, args...))
	}
}

func (impl *impl) Warnf(template string, args ...interface{}) {
	if impl.shouldLog(WARN) {
		impl.log(impl.formatf(WARN, template, args...))
	}
}

func (impl *impl) Warnw(msg string, keysAndValues ...interface{}) {
	if impl.shouldLog(WARN) {
		impl.log(impl.formatw(WARN, msg, keysAndValues...))
	}
}

func (impl *impl) Error(args ...interface{}) {
	if impl.shouldLog(ERROR) {
		impl.log(impl.format(ERROR, args...))
	}
}

func (impl *impl) Errorf(template string, args ...interface{}) {
	if impl.shouldLog(ERROR) {
		impl.log(impl.formatf(ERROR, template, args...))
	}
}

func (impl *impl) Errorw(msg string, keysAndValues ...interface{}) {
	if impl.shouldLog(ERROR) {
		impl.log(impl.formatw(ERROR, msg, keysAndValues...))
	}
}

// These Fatal* methods log as errors then exit the process.
func (impl *impl) Fatal(args ...interface{}) {
	impl.log(impl.format(ERROR, args...))
	os.Exit(1)
}

func (impl *impl) Fatalf(template string, args ...interface{}) {
	impl.log(impl.formatf(ERROR, template, args...))
	os.Exit(1)
}

func (impl *impl) Fatalw(msg string, keysAndValues ...interface{}) {
	impl.log(impl.formatw(ERROR, msg, keysAndValues...))
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
