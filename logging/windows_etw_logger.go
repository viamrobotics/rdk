//go:build windows

package logging

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"go.uber.org/zap/zapcore"
)

// providerGUID is the pinned ETW provider ID. Don't change this — consumers
// (PerfView/tracerpt sessions, dashboards, scripts) key off the GUID. The
// provider name is decorative; the GUID is the stable identifier.
const providerGUID = "66AFF7FE-2451-47AA-A0E3-8E3D2E432B30"

const (
	etwSessionName      = "viam-agent-trace"
	etwDefaultMaxSizeMB = 512
	etwLogmanTimeout    = 30 * time.Second
)

// RegisterETWLogger registers an ETW provider with the pinned GUID, attaches
// it as an Appender on rootLogger, and starts a logman-managed ETW session
// that captures the provider's events into etlPath. Returns an io.Closer
// that stops the session and unregisters the provider; the caller
// defer-closes it.
//
// On any failure during registration, logs via rootLogger and returns a
// no-op closer. The existing eventlog appender is unaffected, so logs still
// reach Event Viewer regardless of ETW health.
func RegisterETWLogger(rootLogger Logger, name, etlPath string) io.Closer {
	g, err := guid.FromString(providerGUID)
	if err != nil {
		rootLogger.Errorw("invalid pinned ETW provider GUID", "err", err)
		return nopCloser{}
	}

	provider, err := etw.NewProviderWithID(name, g, nil)
	if err != nil {
		rootLogger.Errorw("unable to register ETW provider", "err", err)
		return nopCloser{}
	}

	sess := &logmanSessionController{
		name:         etwSessionName,
		providerGUID: providerGUID,
		outputPath:   etlPath,
		maxSizeMB:    etwDefaultMaxSizeMB,
	}

	startCtx, cancel := context.WithTimeout(context.Background(), etwLogmanTimeout)
	defer cancel()

	var liveSession sessionController
	if err := sess.Start(startCtx); err != nil {
		rootLogger.Warnw("ETW session start failed; provider registered but events not captured to file",
			"err", err, "session", etwSessionName, "outputPath", etlPath)
	} else {
		liveSession = sess
	}

	a := &etwAppender{provider: provider, session: liveSession}
	rootLogger.AddAppender(a)
	return a
}

// etwAppender writes each zap entry as a single ETW event with TraceLogging
// fields. Synchronous WriteEvent is sub-microsecond when a session is
// listening, near-free when not — no buffering goroutine needed.
type etwAppender struct {
	provider *etw.Provider
	session  sessionController // nil if session start failed
}

// Write maps the zap entry to a level-tagged ETW event with structured fields.
// Null bytes in the message are scrubbed because Go panics converting
// null-bearing strings to UTF-16 (same reason the eventlog appender does it).
func (a *etwAppender) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	msg := strings.ReplaceAll(entry.Message, "\x00", "NUL")

	fieldsJSON := ""
	if len(fields) > 0 {
		if encoded, err := ZapcoreFieldsToJSON(fields); err == nil {
			fieldsJSON = encoded
		}
	}

	caller := ""
	if entry.Caller.Defined {
		caller = callerToString(&entry.Caller)
	}

	return a.provider.WriteEvent("LogEntry",
		[]etw.EventOpt{etw.WithLevel(zapToETWLevel(entry.Level))},
		[]etw.FieldOpt{
			etw.StringField("time", entry.Time.UTC().Format(DefaultTimeFormatStr)),
			etw.StringField("level", entry.Level.String()),
			etw.StringField("logger", entry.LoggerName),
			etw.StringField("caller", caller),
			etw.StringField("message", msg),
			etw.JSONStringField("fields", fieldsJSON),
		},
	)
}

func (a *etwAppender) Sync() error { return nil }

// Close stops the session before unregistering the provider so any kernel
// buffer contents are flushed to the .etl file before teardown.
func (a *etwAppender) Close() error {
	var sessErr error
	if a.session != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), etwLogmanTimeout)
		defer cancel()
		sessErr = a.session.Stop(stopCtx)
	}
	provErr := a.provider.Close()
	if sessErr != nil {
		return sessErr
	}
	return provErr
}

func zapToETWLevel(l zapcore.Level) etw.Level {
	switch l {
	case zapcore.DebugLevel, zapcore.InfoLevel:
		return etw.LevelInfo
	case zapcore.WarnLevel:
		return etw.LevelWarning
	case zapcore.ErrorLevel:
		return etw.LevelError
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return etw.LevelCritical
	default:
		return etw.LevelInfo
	}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
