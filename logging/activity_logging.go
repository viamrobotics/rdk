package logging

import (
	"time"
)

// activityLoggerName is the reserved logger name for activity logs.
// server and agent share this logger.
const activityLoggerName = "rdk.activity"

// activityLogger holds the state behind the package-level Activity functions. impl is held as a
// field rather than embedded so its Debug/Info/Warn/Error methods are not reachable; activity
// logs can only be emitted through Activity and ActivityError.
type activityLogger struct {
	logger *impl
	// unit names the emitting process and is stamped on every event.
	unit string
}

func newActivityLogger(registry *Registry, unit string) *activityLogger {
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
	return &activityLogger{logger: logger, unit: unit}
}

// globalActivityLogger starts with no sinks (Activity calls are dropped) until
// InitActivityLogger installs one registered against the server's logger registry.
var globalActivityLogger = newActivityLogger(newRegistry(), "unknown")

// InitActivityLogger installs the process-wide activity logger. unit names the emitting process
// (e.g. "server", "agent") and is stamped on every event. Registering into the given registry
// gives the activity logger the same net and local (offline) appenders as the rest of the
// logger tree via AddAppenderToAll. Call once, early in startup, before any Activity callers.
func InitActivityLogger(registry *Registry, unit string) {
	globalActivityLogger = newActivityLogger(registry, unit)
}

// Activity emits an activity log. It always writes at INFO regardless of any configured level
// and is never deduplicated, so every activity event reaches the sink. eventType is the
// subsystem noun (e.g. "reconfigure", "network", "process"); event is the transition verb
// (e.g. "start", "success", "failure", "connect").
// Callers must not set "unit", "event_type", or "event" in keysAndValues.
//
// The log body lives here rather than in a helper so the call depth matches the standard
// logger methods and getCaller attributes the entry to the Activity call site.
func Activity(eventType, event string, keysAndValues ...any) {
	al := globalActivityLogger
	// Prepend so unit, event_type, and event lead the rendered fields.
	keysAndValues = append([]any{"unit", al.unit, "event_type", eventType, "event", event}, keysAndValues...)
	entry := al.logger.formatw(INFO, emptyTraceKey, "", keysAndValues...)
	al.logger.Write(entry)
}

// ActivityError emits an activity event at ERROR severity, for events that are bad outcomes
// (e.g. "fail"). Emission is identical to Activity: unconditional and never deduplicated.
//
// The body is duplicated from Activity rather than shared so the call depth matches and
// getCaller attributes the entry to the ActivityError call site.
func ActivityError(eventType, event string, keysAndValues ...any) {
	al := globalActivityLogger
	// Prepend so unit, event_type, and event lead the rendered fields.
	keysAndValues = append([]any{"unit", al.unit, "event_type", eventType, "event", event}, keysAndValues...)
	entry := al.logger.formatw(ERROR, emptyTraceKey, "Event:", keysAndValues...)
	al.logger.Write(entry)
}
