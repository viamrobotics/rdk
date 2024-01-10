package logging

import (
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

type testAppender struct {
	tb testing.TB
}

// NewTestAppender returns a logger appender that logs to the underlying `testing.TB` object.
func NewTestAppender(tb testing.TB) Appender {
	return &testAppender{tb}
}

// Write outputs the log entryt o the underlying test object `Log` method.
func (tapp *testAppender) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	tapp.tb.Helper()
	const maxLength = 10
	toPrint := make([]string, 0, maxLength)
	// toPrint = append(toPrint, entry.Time.Format(appender.timeFormatStr))
	toPrint = append(toPrint, entry.Time.Format("2006-01-02T15:04:05.000Z0700"))

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

func (tapp *testAppender) Sync() error {
	return nil
}
