package logging

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// activityLoggerName is the reserved logger name for activity logs. The cloud sink filters
// activity out of general machine logs by matching on this name, so it must be kept in sync
// with the backend. viam-agent emits activity to the same name against the same part.
const activityLoggerName = "rdk.activity"

// ActivityLogger is a deliberately narrow logging surface. It exposes only Event, so the
// standard Debug/Info/Warn/Error methods cannot be called on it and activity logs can only be
// emitted through the fixed Event signature. The emitting unit (e.g. "server", "agent") is
// stamped at construction rather than passed per call.
type ActivityLogger interface {
	Event(eventType, event string, keysAndValues ...any)
}

// activityLogger backs ActivityLogger. impl is held as an unexported field rather than embedded,
// so none of impl's Debug/Info/Warn/Error methods are promoted onto the exposed type.
type activityLogger struct {
	logger *impl
	// unitField is the precomputed "unit" field stamped on every event.
	unitField zapcore.Field
}

// Event emits an activity log. It always writes at INFO regardless of any configured level
// and is never deduplicated, so every activity event reaches the sink. eventType is the
// subsystem noun (e.g. "reconfigure", "network", "process"); event is the transition verb
// (e.g. "start", "success", "failure", "connect").
// Callers must not set "unit", "event_type", or "event" in keysAndValues.
func (a *activityLogger) Event(eventType, event string, keysAndValues ...any) {
	keysAndValues = append(keysAndValues, "event_type", eventType, "event", event)
	entry := a.logger.formatw(INFO, emptyTraceKey, "Activity event:", keysAndValues...)
	entry.Fields = append(entry.Fields, a.unitField)
	a.logger.Write(entry)
}

// NewActivityLogger builds a fully-initialized activity logger named rdk.activity with
// deduplication disabled. unit names the emitting process (e.g. "server", "agent") and is
// stamped on every event. It registers into the given registry so that AddAppenderToAll gives
// it the same net and local (offline) appenders as the rest of the logger tree.
func NewActivityLogger(registry *Registry, unit string) ActivityLogger {
	logger := &impl{
		name:                     activityLoggerName,
		level:                    NewAtomicLevelAt(INFO),
		registry:                 registry,
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}
	logger.NeverDeduplicate()
	registry.registerLogger(activityLoggerName, logger)
	return &activityLogger{logger: logger, unitField: zap.String("unit", unit)}
}

// globalActivityLogger is the process-wide activity logger that Activity writes to. It starts
// with no sinks (Event calls are dropped) until SetGlobalActivityLogger installs one registered
// against the server's logger registry at startup.
var globalActivityLogger ActivityLogger = NewActivityLogger(newRegistry(), "unknown")

// SetGlobalActivityLogger installs the process-wide activity logger. It should be called once,
// early in startup, before any Activity callers run.
func SetGlobalActivityLogger(l ActivityLogger) {
	globalActivityLogger = l
}

// Activity emits an activity log through the process-wide activity logger.
func Activity(eventType, event string, keysAndValues ...any) {
	globalActivityLogger.Event(eventType, event, keysAndValues...)
}

