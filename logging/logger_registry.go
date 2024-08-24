package logging

import (
	"fmt"
	"regexp"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	loggers   map[string]Logger
	logConfig []LoggerPatternConfig
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

func (lr *Registry) deregisterLogger(name string) bool {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	_, ok := lr.loggers[name]
	if ok {
		delete(lr.loggers, name)
	}
	return ok
}

func (lr *Registry) loggerNamed(name string) (logger Logger, ok bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	logger, ok = lr.loggers[name]
	return
}

func (lr *Registry) updateLoggerLevelWithCfg(name string) error {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	for _, lpc := range lr.logConfig {
		r, err := regexp.Compile(buildRegexFromPattern(lpc.Pattern))
		if err != nil {
			return err
		}
		if r.MatchString(name) {
			logger, ok := lr.loggers[name]
			if !ok {
				return fmt.Errorf("logger named %s not recognized", name)
			}
			level, err := LevelFromString(lpc.Level)
			if err != nil {
				return err
			}
			logger.SetLevel(level)
		}
	}

	return nil
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

func (lr *Registry) UpdateConfig(logConfig []LoggerPatternConfig, errorLogger Logger) error {
	lr.mu.Lock()
	lr.logConfig = logConfig
	lr.mu.Unlock()

	appliedConfigs := make(map[string]Level)
	for _, lpc := range logConfig {
		if !validatePattern(lpc.Pattern) {
			errorLogger.Warnw("failed to validate a pattern", "pattern", lpc.Pattern)
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
			level = INFO
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
	registeredNames := make([]string, 0, len(globalLoggerRegistry.loggers))
	for name := range lr.loggers {
		registeredNames = append(registeredNames, name)
	}
	return registeredNames
}

func (lr *Registry) getCurrentConfig() []LoggerPatternConfig {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	return lr.logConfig
}

// getOrRegister will either:
//   - return an existing logger for the input logger `name` or
//   - register the input `logger` for the given logger `name` and configure it based on the
//     existing patterns.
//
// Such that if concurrent callers try registering the same logger, the "winner"s logger will be
// registered and all losers will return the winning logger.
//
// It is expected in racing scenarios that all callers are trying to register behaviorly equivalent
// `logger` objects.
func (lr *Registry) getOrRegister(name string, logger Logger) Logger {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if existingLogger, ok := lr.loggers[name]; ok {
		return existingLogger
	}

	lr.loggers[name] = logger
	lr.updateLoggerLevelWithCfg(name)
	return logger
}
