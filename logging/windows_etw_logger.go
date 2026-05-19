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

// ETW provider GUID used by all ETW log consumers - should not be changed
const providerGUID = "66AFF7FE-2451-47AA-A0E3-8E3D2E432B30"

const (
	etwSessionName      = "viam-server-trace"
	etwDefaultMaxSizeMB = 512
	etwLogmanTimeout    = 30 * time.Second
)

// RegisterETWLogger registers an ETW provider with the pinned GUID, attaches
// it as an Appender on rootLogger, and starts a logman-managed ETW session
// that captures the provider's events into etlPath
func RegisterETWLogger(rootLogger Logger, name, etlPath string) (io.Closer, error) {
	g, err := guid.FromString(providerGUID)
	if err != nil {
		rootLogger.Errorw("invalid pinned ETW provider GUID", "err", err)
		return nopCloser{}, err
	}

	provider, err := etw.NewProviderWithID(name, g, nil)
	if err != nil {
		rootLogger.Errorw("unable to register ETW provider", "err", err)
		return nopCloser{}, err
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
		rootLogger.Warnw("ETW session start failed; provider registered but file capture could not start",
			"err", err, "session", etwSessionName, "outputPath", etlPath)
		provider.Close()
		return nopCloser{}, err
	} else {
		liveSession = sess
	}

	a := &etwAppender{provider: provider, session: liveSession}
	rootLogger.AddAppender(a)
	return a, nil
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
	// if the caller isn't defined, assume it's at the beginning of the log line
	if entry.Caller.Defined {
		caller = callerToString(&entry.Caller)
	} else {
		prefix, after, found := strings.Cut(msg, "\t")
		if found {
			caller = prefix
			msg = after
		}
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
