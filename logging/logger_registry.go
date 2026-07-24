package logging

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

// ActivityLoggerName is the reserved logger name prefix for activity logs. Each
// registry's activity logger is named ActivityLoggerName + "." + unit (e.g.
// "rdk.activity.server"); the cloud identifies activity by this prefix, so it must be
// kept in sync with the backend. viam-server and viam-agent emit under the same prefix
// against the same part.
const ActivityLoggerName = "rdk.activity"

// Registry is a registry of loggers. It is stored on a logger, and holds a map
// of known subloggers (`loggers`) and a slice of configuration objects
// (`logConfig`).
type Registry struct {
	mu        sync.RWMutex
	loggers   map[string]Logger
	logConfig []LoggerPatternConfig

	// activityUnit names the process emitting this registry's activity events (e.g.
	// "server", "agent"); see SetActivityUnit.
	activityUnit string

	// DeduplicateLogs controls whether to deduplicate logs. Slightly odd to store this on
	// the registry but preferable to having a global atomic.
	DeduplicateLogs atomic.Bool
}

func newRegistry() *Registry {
	ret := &Registry{
		loggers: make(map[string]Logger),
	}
	applyMotionRegistryOptions(ret)

	return ret
}

func (lr *Registry) registerLogger(name string, logger Logger) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.loggers[name] = logger
}

// loggerNamed returns the registered logger with the given name, if any.
func (lr *Registry) loggerNamed(name string) (logger Logger, ok bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	logger, ok = lr.loggers[name]
	return
}

func (lr *Registry) updateLoggerLevel(name string, level Level) error {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	logger, ok := lr.loggers[name]
	if !ok {
		return fmt.Errorf("logger named %s not recognized", name)
	}
	logger.SetLevel(level)
	return nil
}

// Update updates the logger registry with the passed in `logConfig`. Invalid patterns
// are warn-logged through the warnLogger.
func (lr *Registry) Update(logConfig []LoggerPatternConfig, warnLogger Logger) {
	lr.mu.Lock()
	lr.logConfig = logConfig
	lr.mu.Unlock()

	appliedConfigs := make(map[string]Level)
	for _, lpc := range logConfig {
		r, err := regexp.Compile(BuildRegexFromPattern(lpc.Pattern))
		if err != nil {
			warnLogger.Warnw("Log regex did not compile",
				"input", lpc.Pattern, "built", BuildRegexFromPattern(lpc.Pattern), "err", err)
			continue
		}

		level, err := LevelFromString(lpc.Level)
		if err != nil {
			warnLogger.Warnw("Log level did not parse", "pattern", lpc.Pattern, "level", lpc.Level)
			continue
		}

		for _, name := range lr.getRegisteredLoggerNames() {
			if r.MatchString(name) {
				appliedConfigs[name] = level
			}
		}
	}

	for _, name := range lr.getRegisteredLoggerNames() {
		level, ok := appliedConfigs[name]
		if !ok {
			// If no config was applied; return logger to level of passed in
			// warnLogger. Idea being that if _no_ config applies to logger
			// anymore, warnLogger should be the logger from entrypoint and
			// therefore the highest in the tree of loggers.
			level = warnLogger.GetLevel()
		}
		err := lr.updateLoggerLevel(name, level)
		if err != nil {
			warnLogger.Warnw("Logger disappeared after seeing its name", "name", name, "level", level)
		}
	}
}

func (lr *Registry) getRegisteredLoggerNames() []string {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	registeredNames := make([]string, 0, len(lr.loggers))
	for name := range lr.loggers {
		registeredNames = append(registeredNames, name)
	}
	return registeredNames
}

// GetCurrentConfig gets the current config.
func (lr *Registry) GetCurrentConfig() []LoggerPatternConfig {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	return lr.logConfig
}

// AddAppenderToAll adds the specified appender to all loggers in the registry.
func (lr *Registry) AddAppenderToAll(appender Appender) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	for _, logger := range lr.loggers {
		logger.AddAppender(appender)
	}
}

// SetActivityUnit sets the unit segment of this registry's activity logger name
// (rdk.activity.<unit>), identifying the emitting process. It eagerly creates the
// activity logger so that subsequent AddAppenderToAll calls give it the same sinks as
// the rest of the logger tree. Call once, early in startup, before appenders are
// attached and before any Activity callers.
func (lr *Registry) SetActivityUnit(unit string) {
	lr.mu.Lock()
	lr.activityUnit = unit
	lr.mu.Unlock()
	lr.activityLogger()
}

// ActivityLogger returns this registry's activity logger, creating and registering it
// if needed. Events should be emitted through Logger.Activity rather than this logger's
// own methods; it is exposed for attaching appenders.
func (lr *Registry) ActivityLogger() Logger {
	return lr.activityLogger()
}

func (lr *Registry) activityLogger() *impl {
	lr.mu.RLock()
	unit := lr.activityUnit
	lr.mu.RUnlock()
	if unit == "" {
		unit = "unknown"
	}
	name := ActivityLoggerName + "." + unit

	if logger, ok := lr.loggerNamed(name); ok {
		//nolint:forcetypeassert
		return logger.(*impl)
	}
	logger := &impl{
		name:                     name,
		level:                    NewAtomicLevelAt(INFO),
		registry:                 lr,
		testHelper:               func() {},
		recentMessageCounts:      make(map[string]int),
		recentMessageEntries:     make(map[string]LogEntry),
		recentMessageWindowStart: time.Now(),
	}
	logger.NeverDeduplicate()
	// Only this function registers loggers under the activity name, so the registered
	// logger is always an *impl.
	//nolint:forcetypeassert
	return lr.getOrRegister(name, logger).(*impl)
}

// getOrRegister will either:
//   - return an existing logger for the input logger `name` or
//   - register the input `logger` for the given logger `name` and configure it based on the
//     existing patterns.
//
// Such that if concurrent callers try registering the same logger, the "winner"s logger will be
// registered and all losers will return the winning logger.
//
// It is expected in racing scenarios that all callers are trying to register behavioral equivalent
// `logger` objects.
func (lr *Registry) getOrRegister(name string, logger Logger) Logger {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if existingLogger, ok := lr.loggers[name]; ok {
		return existingLogger
	}

	lr.loggers[name] = logger
	for _, lpc := range lr.logConfig {
		r, err := regexp.Compile(BuildRegexFromPattern(lpc.Pattern))
		if err != nil {
			// Can ignore error here; invalid pattern will already have been
			// warn-logged as part of config reading.
			continue
		}
		if r.MatchString(name) {
			level, err := LevelFromString(lpc.Level)
			if err != nil {
				// Can ignore error here; invalid level will already have been
				// warn-logged as part of config reading.
				continue
			}
			logger.SetLevel(level)
		}
	}
	return logger
}
