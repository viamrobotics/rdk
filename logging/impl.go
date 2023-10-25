package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type impl struct {
	name  string
	level Level

	out io.Writer
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

func (impl *impl) log(msg string) {
	fmt.Fprintln(impl.out, msg)
}

func (impl *impl) getPrefix(logLevel Level) string {
	const prefixLength = 6
	toPrint := make([]any, prefixLength)
	toPrint[0] = time.Now().Format("2006-01-02T15:04:05.000Z0700")
	toPrint[1] = "\t"
	toPrint[2] = strings.ToUpper(logLevel.String())
	toPrint[3] = "\t"
	toPrint[4] = getCaller()
	toPrint[5] = "\t"

	return fmt.Sprint(toPrint...)
}

// Forwards straight to `fmt.Sprint` without any formatting.
func (impl *impl) format(logLevel Level, args ...interface{}) string {
	// 2023-10-25T15:57:11.979-0400	INFO	zap	logging/impl_test.go:36	Info log
	toPrint := make([]any, len(args)+1)
	toPrint[0] = impl.getPrefix(logLevel)
	for idx := 0; idx < len(args); idx++ {
		toPrint[idx+1] = args[idx]
	}

	return fmt.Sprint(toPrint...)
}

// Forwards to `fmt.Sprintf` which does standard Go formatting.
func (impl *impl) formatf(logLevel Level, template string, args ...interface{}) string {
	toPrint := impl.getPrefix(logLevel)
	logValue := fmt.Sprintf(template, args...)

	return fmt.Sprintf("%s%s", toPrint, logValue)
}

// Turns `keysAndValues` into a map where the odd elements are the keys and their following even
// counterpart is the value. The keys are expected to be strings. The values are json
// serialized. Only public fields are included in the serialization.
func (impl *impl) formatw(logLevel Level, msg string, keysAndValues ...interface{}) string {
	prefix := impl.getPrefix(logLevel)
	mp := make(map[string]any)
	for keyIdx := 0; keyIdx < len(keysAndValues); keyIdx += 2 {
		key := keysAndValues[keyIdx]
		var val any
		if keyIdx+1 < len(keysAndValues) {
			val = keysAndValues[keyIdx+1]
		}

		if stringer, ok := key.(fmt.Stringer); ok {
			mp[stringer.String()] = val
		} else {
			mp[fmt.Sprintf("%v", key)] = val
		}
	}

	// Zap structured logging omits private fields. Thus JSON marshalling should be sufficient.
	var encMapStr string
	encMapBytes, err := json.Marshal(mp)
	if err == nil {
		encMapStr = string(encMapBytes)
	} else {
		encMapStr = fmt.Sprintf("%v", mp)
	}

	// The prefix has a trailing tab character. Directly concatenate the `prefix` with the `msg`.
	return fmt.Sprintf("%s%s\t%s", prefix, msg, encMapStr)
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

// Return example: "logging/impl_test.go:36".
func getCaller() string {
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		return "unknown_file.go:0"
	}

	// The file returned by `runtime.Caller` is a full path and always contains '/' to separate
	// directories. Including on windows. We only want to keep the `<package>/<file>` part of the
	// path. We use a stateful lambda to count back two '/' runes.
	cnt := 0
	idx := strings.LastIndexFunc(file, func(rn rune) bool {
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
	return fmt.Sprintf("%s:%d", file[idx+1:], line)
}
