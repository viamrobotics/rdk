//go:build windows

package logging

import (
	"strings"

	"go.uber.org/zap/zapcore"
	"golang.org/x/sys/windows/svc/eventlog"
)

// RegisterEventLogger does nothing on Unix. On Windows it will add an `Appender` for logging to
// windows event system.
func RegisterEventLogger(rootLogger Logger) {
	log, err := eventlog.Open("viam-server")
	if err != nil {
		rootLogger.Errorw("Unable to open windows event log", "err", err)
	}
	rootLogger.AddAppender(&eventLogger{log})
}

type eventLogger struct {
	log *eventlog.Log
}

func getMessage(entry zapcore.Entry, fields []zapcore.Field) string {
	const maxLength = 10
	toPrint := make([]string, 0, maxLength)
	// We use UTC such that logs from different `viam-server`s can have their logs compared without
	// needing them to be configured in the same timezone.
	toPrint = append(toPrint, entry.Time.UTC().Format(DefaultTimeFormatStr))
	toPrint = append(toPrint, strings.ToUpper(entry.Level.String()))
	toPrint = append(toPrint, entry.LoggerName)
	if entry.Caller.Defined {
		toPrint = append(toPrint, callerToString(&entry.Caller))
	}
	toPrint = append(toPrint, entry.Message)
	if len(fields) == 0 {
		return strings.Join(toPrint, "\t")
	}

	fieldsJSON, err := ZapcoreFieldsToJSON(fields)
	if err != nil {
		return strings.Join(toPrint, "\t")
	}
	toPrint = append(toPrint, fieldsJSON)

	return strings.Join(toPrint, "\t")
}

func (el *eventLogger) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	switch entry.Level {
	case zapcore.DebugLevel, zapcore.InfoLevel:
		el.log.Info(0, getMessage(entry, fields))
	case zapcore.WarnLevel:
		el.log.Warning(0, getMessage(entry, fields))
	default: // includes zapcore.ErrorLevel and "more threatening" levels
		el.log.Error(0, getMessage(entry, fields))
	}
	return nil
}

func (el *eventLogger) Sync() error {
	return nil
}
