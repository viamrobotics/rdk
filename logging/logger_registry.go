package logging

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
)

// Registry is a registry of loggers. It is stored on a logger, and holds a map
// of known subloggers (`loggers`) and a slice of configuration objects
// (`logConfig`).
type Registry struct {
	mu        sync.RWMutex
	loggers   map[string]Logger
	logConfig []LoggerPatternConfig

	// DeduplicateLogs controls whether to deduplicate logs. Slightly odd to store this on
	// the registry but preferable to having a global atomic.
	DeduplicateLogs atomic.Bool
}

func newRegistry() *Registry {
	return &Registry{
		loggers: make(map[string]Logger),
	}
}

func (lr *Registry) registerLogger(name string, logger Logger) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.loggers[name] = logger
}

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
func (lr *Registry) Update(logConfig []LoggerPatternConfig, warnLogger Logger) error {
	lr.mu.Lock()
	lr.logConfig = logConfig
	lr.mu.Unlock()

	appliedConfigs := make(map[string]Level)
	for _, lpc := range logConfig {
		if !validatePattern(lpc.Pattern) {
			warnLogger.Warnw("failed to validate a pattern", "pattern", lpc.Pattern)
			continue
		}

		r, err := regexp.Compile(buildRegexFromPattern(lpc.Pattern))
		if err != nil {
			return err
		}

		for _, name := range lr.getRegisteredLoggerNames() {
			if r.MatchString(name) {
				level, err := LevelFromString(lpc.Level)
				if err != nil {
					return err
				}
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
			return err
		}
	}

	return nil
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
		r, err := regexp.Compile(buildRegexFromPattern(lpc.Pattern))
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
