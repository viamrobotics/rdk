//go:build windows

package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"go.uber.org/zap/zapcore"
)

// ETW provider GUID used by all ETW log consumers - should not be changed
const providerGUID = "66AFF7FE-2451-47AA-A0E3-8E3D2E432B30"

const (
	etwSessionName        = "viam-server-trace"
	etwDefaultMaxSizeMB   = 64
	etwDefaultTotalSizeMB = 448
	etwLogmanTimeout      = 30 * time.Second

	// etwFileTimeFormat is the timestamp embedded into per-session .etl
	// filenames. Hyphens (not colons) so the path is valid on Windows.
	etwFileTimeFormat = "2006-01-02T15-04-05"
)

// RegisterETWLogger registers an ETW provider with the pinned GUID, attaches
// it as an Appender on rootLogger, and starts a logman-managed ETW session
// that captures the provider's events to a timestamped .etl file inside
// etlDir.
//
// Before starting the new session, older session files in etlDir matching
// the etwSessionName-*.etl naming pattern are pruned (oldest first) until
// total bytes fit under maxTotalSizeMB
func RegisterETWLogger(rootLogger Logger, name, etlDir string) (io.Closer, error) {
	if err := os.MkdirAll(etlDir, 0o755); err != nil {
		rootLogger.Warnw("could not create ETL directory", "dir", etlDir, "err", err)
	}
	pruneOldETLFiles(rootLogger, etlDir, int64(etwDefaultTotalSizeMB)*1024*1024)

	etlPath := filepath.Join(etlDir,
		fmt.Sprintf("%s-%s.etl", etwSessionName, time.Now().UTC().Format(etwFileTimeFormat)))

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

// pruneOldETLFiles enforces a total-bytes budget across all retained
// session files in dir. Files matching <etwSessionName>-*.etl are
// considered; the oldest (by mtime) are deleted first until total bytes
// fit under maxTotalBytes. Errors are logged and otherwise ignored.
func pruneOldETLFiles(rootLogger Logger, dir string, maxTotalBytes int64) {
	matches, err := filepath.Glob(filepath.Join(dir, etwSessionName+"-*.etl"))
	if err != nil || len(matches) == 0 {
		return
	}

	type entry struct {
		path  string
		size  int64
		mtime time.Time
	}
	var entries []entry
	var total int64
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		entries = append(entries, entry{m, st.Size(), st.ModTime()})
		total += st.Size()
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].mtime.Before(entries[j].mtime)
	})

	for total > maxTotalBytes && len(entries) > 0 {
		e := entries[0]
		entries = entries[1:]
		if err := os.Remove(e.path); err != nil {
			rootLogger.Warnw("could not delete old ETL file", "path", e.path, "err", err)
			continue
		}
		total -= e.size
	}
}
