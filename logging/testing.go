package logging

import (
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

type testAppender struct {
	tb testing.TB
}

// NewTestAppender returns a logger appender that logs to the underlying `testing.TB`
// object. Writing logs with `tb.Log` does two things:
//   - Prepends the log with the filename/line number that called the `tb.Log` method. This is not
//     useful to us.
//   - Correctly associates the log line with a Golang "Test*" function.
//
// Additionally, this test appender will log in the local/machine timezone.
//
// For tests that run in series (i.e: do not call `t.Parallel()`), writing to stdout (i.e:
// `fmt.Print`) this does not matter. Go's best effort detection works fine. But for tests running
// in parallel, the best-effort algorithm can map log lines to the wrong test. This is most
// noticeable when json logging is enabled. Where each log line has its test name, e.g:
//
// {"Time":"2024-01-23T09:26:57.843619918-05:00","Action":"output","Package":"go.viam.com/rdk/robot","Test":"TestSessions/window_size=2s","Output":"local_robot.go:760: 2024-01-23T09:26:57.843-0500\tDEBUG\t\timpl/local_robot.go:760\thandlingweak update for..."}
//
// A note about the filename/line that Go prepends to these test logs. Go normally finds this by
// walking up a couple stack frames to discover where a call to `t.Log` came from. However, it's
// not our desire for that to refer to the `tapp.tb.Log` call from the `testAppender.Write` method
// below. Go exposes a `tb.Helper()` method for this. All functions that call `tb.Helper` will add
// their stack frame to a filter. Go will walk up and report the first filename/line number from the
// first method that does not exempt itself.
//
// Notably, zap's testing logger has a bug in this regard. Due to forgetting a `tb.Helper` call,
// test logs through their library always include `logger.go:130` instead of the actual log line.
//
//nolint:lll
func NewTestAppender(tb testing.TB) Appender {
	return &testAppender{tb}
}

// Write outputs the log entry to the underlying test object `Log` method.
func (tapp *testAppender) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	tapp.tb.Helper()
	const maxLength = 10
	toPrint := make([]string, 0, maxLength)
	toPrint = append(toPrint, entry.Time.Format(DefaultTimeFormatStr))

	toPrint = append(toPrint, strings.ToUpper(entry.Level.String()))
	toPrint = append(toPrint, entry.LoggerName)
	if entry.Caller.Defined {
		toPrint = append(toPrint, callerToString(&entry.Caller))
	}
	toPrint = append(toPrint, entry.Message)
	if len(fields) == 0 {
		tapp.tb.Log(strings.Join(toPrint, "\t"))
		return nil
	}

	// Use zap's json encoder which will encode our slice of fields in-order. As opposed to the
	// random iteration order of a map. Call it with an empty Entry object such that only the fields
	// become "map-ified".
	jsonEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{SkipLineEnding: true})
	buf, err := jsonEncoder.EncodeEntry(zapcore.Entry{}, fields)
	if err != nil {
		// Log what we have and return the error.
		tapp.tb.Log(strings.Join(toPrint, "\t"))
		return err
	}
	toPrint = append(toPrint, string(buf.Bytes()))
	tapp.tb.Log(strings.Join(toPrint, "\t"))
	return nil
}

// Sync is a no-op.
func (tapp *testAppender) Sync() error {
	return nil
}
