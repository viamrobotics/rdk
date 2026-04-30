package logging

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Log-path stopwatch. Enabled when VIAM_LOG_STOPWATCH is set to a writable
// file path. Both the module process and the parent viam-server process append
// tagged, fixed-width lines to that file. Grep by tag or id to interpret.
//
// Line tags:
//
//	MLOG — emitted on the module side (one line per logger.Infof etc.)
//	PLOG — emitted on the parent side (one line per Server.Log RPC)
//	PAPP — emitted on the parent side, one line per appender in the fanout
//
// Correlate MLOG with PLOG/PAPP via the id field.
//
// Example output:
//
//	2026-04-20T14:23:17.123456Z MLOG id=0000000042 encode=   12µs rpc=   87.4ms total=   87.4ms lvl=info msg="hello"
//	2026-04-20T14:23:17.205123Z PLOG id=0000000042 fanout=    5.1ms napp=3                      lvl=info msg="hello"
//	2026-04-20T14:23:17.205456Z PAPP id=0000000042 appender=logging.ConsoleAppender  dur=   0.4ms
//	2026-04-20T14:23:17.205789Z PAPP id=0000000042 appender=*logging.NetAppender     dur=   0.1ms
//	2026-04-20T14:23:17.206012Z PAPP id=0000000042 appender=*logging.eventLogger     dur=   4.0ms

const logStopwatchEnvVar = "VIAM_LOG_STOPWATCH"

// Metadata key used on the module->parent gRPC call to propagate the trace id.
const LogStopwatchMetadataKey = "x-viam-logtrace-id"

var (
	logStopwatchInit sync.Once
	logStopwatchFile *os.File
	logStopwatchMu   sync.Mutex
	logStopwatchSeq  atomic.Uint64
)

// LogStopwatchEnabled reports whether VIAM_LOG_STOPWATCH is set and the file
// was opened successfully.
func LogStopwatchEnabled() bool {
	logStopwatchInit.Do(openLogStopwatch)
	return logStopwatchFile != nil
}

func openLogStopwatch() {
	path := os.Getenv(logStopwatchEnvVar)
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logtrace: cannot open %s: %v\n", path, err)
		return
	}
	logStopwatchFile = f
}

// NextLogStopwatchID returns a fresh zero-padded correlation id. Ids are
// monotonic per-process; combine with the (module-side) module name in your
// grep query if multiple modules run concurrently.
func NextLogStopwatchID() string {
	return fmt.Sprintf("%010d", logStopwatchSeq.Add(1))
}

// LogStopwatchWrite writes one tagged line. kv pairs are rendered as
// key=value separated by single spaces. Durations pass through FormatDur for
// fixed-width alignment; strings are quoted only when they contain spaces or
// quotes.
func LogStopwatchWrite(tag string, kv ...any) {
	if !LogStopwatchEnabled() {
		return
	}
	if len(kv)%2 != 0 {
		kv = append(kv, "MISSING_VALUE")
	}

	var b strings.Builder
	b.Grow(256)
	b.WriteString(time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"))
	b.WriteByte(' ')
	b.WriteString(tag)
	for i := 0; i < len(kv); i += 2 {
		b.WriteByte(' ')
		fmt.Fprintf(&b, "%v=%s", kv[i], formatValue(kv[i+1]))
	}
	b.WriteByte('\n')

	logStopwatchMu.Lock()
	defer logStopwatchMu.Unlock()
	if logStopwatchFile != nil {
		_, _ = logStopwatchFile.WriteString(b.String())
	}
}

// FormatDur returns a right-aligned human-readable duration, rounded to
// microseconds. Width is 9 characters so columns line up across lines:
//
//	"    12µs", "  87.4ms", "    1.2s"
func FormatDur(d time.Duration) string {
	return fmt.Sprintf("%9s", d.Round(time.Microsecond).String())
}

// RawValue is a string that LogStopwatchWrite emits verbatim, skipping the
// auto-quoting applied to ordinary strings that contain whitespace. Use it
// for values whose layout you control (e.g. pre-padded column names).
type RawValue string

// FormatAppenderName right-pads the concrete type name of an appender so PAPP
// lines align in column. Returns a RawValue so trailing padding is preserved
// without being quoted.
func FormatAppenderName(a Appender) RawValue {
	name := fmt.Sprintf("%T", a)
	const width = 32
	if len(name) >= width {
		return RawValue(name)
	}
	return RawValue(name + strings.Repeat(" ", width-len(name)))
}

func formatValue(v any) string {
	switch vv := v.(type) {
	case time.Duration:
		return FormatDur(vv)
	case RawValue:
		return string(vv)
	case string:
		if strings.ContainsAny(vv, " \t\"") {
			return fmt.Sprintf("%q", vv)
		}
		return vv
	default:
		s := fmt.Sprintf("%v", vv)
		if strings.ContainsAny(s, " \t\"") {
			return fmt.Sprintf("%q", s)
		}
		return s
	}
}
