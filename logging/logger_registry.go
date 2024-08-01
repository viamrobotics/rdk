package logging

import (
	"fmt"
	"regexp"
	"sync"
)

type loggerRegistry struct {
	mu        sync.RWMutex
	loggers   map[string]Logger
	logConfig []LoggerPatternConfig
}

// TODO(RSDK-8250): convert loggerManager from global variable to variable on local robot.
var globalLoggerRegistry = newLoggerRegistry()

func newLoggerRegistry() *loggerRegistry {
	return &loggerRegistry{
		loggers: make(map[string]Logger),
	}
}

func (lr *loggerRegistry) registerLogger(name string, logger Logger) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.loggers[name] = logger
}

func (lr *loggerRegistry) deregisterLogger(name string) bool {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	_, ok := lr.loggers[name]
	if ok {
		delete(lr.loggers, name)
	}
	return ok
}

func (lr *loggerRegistry) loggerNamed(name string) (logger Logger, ok bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	logger, ok = lr.loggers[name]
	return
}

func (lr *loggerRegistry) updateLoggerLevelWithCfg(name string) error {
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

func (lr *loggerRegistry) updateLoggerLevel(name string, level Level) error {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	logger, ok := lr.loggers[name]
	if !ok {
		return fmt.Errorf("logger named %s not recognized", name)
	}
	logger.SetLevel(level)
	return nil
}

func (lr *loggerRegistry) updateLoggerRegistry(logConfig []LoggerPatternConfig) error {
	appliedConfigs := make(map[string]Level)
	for _, lpc := range logConfig {
		if !validatePattern(lpc.Pattern) {
			logger := GetOrNewLogger("rdk.logging")
			logger.Info("failed to validate a pattern", "pattern", lpc.Pattern)
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

func (lr *loggerRegistry) getRegisteredLoggerNames() []string {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	registeredNames := make([]string, 0, len(globalLoggerRegistry.loggers))
	for name := range lr.loggers {
		registeredNames = append(registeredNames, name)
	}
	return registeredNames
}

func (lr *loggerRegistry) registerConfig(logConfig []LoggerPatternConfig) error {
	lr.mu.Lock()
	lr.logConfig = logConfig
	lr.mu.Unlock()
	return lr.updateLoggerRegistry(logConfig)
}

func (lr *loggerRegistry) getCurrentConfig() []LoggerPatternConfig {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	return lr.logConfig
}

// Exported Functions specifically for use on global logger manager.

// RegisterLogger registers a new logger with a given name.
func RegisterLogger(name string, logger Logger) {
	globalLoggerRegistry.registerLogger(name, logger)
}

// DeregisterLogger attempts to remove a logger from the registry and returns a boolean denoting whether it succeeded.
func DeregisterLogger(name string) bool {
	return globalLoggerRegistry.deregisterLogger(name)
}

// LoggerNamed returns logger with specified name if exists.
func LoggerNamed(name string) (logger Logger, ok bool) {
	return globalLoggerRegistry.loggerNamed(name)
}

// UpdateLoggerLevel assigns level to appropriate logger in the registry.
func UpdateLoggerLevel(name string, level Level) error {
	return globalLoggerRegistry.updateLoggerLevel(name, level)
}

// GetRegisteredLoggerNames returns the names of all loggers in the registry.
func GetRegisteredLoggerNames() []string {
	return globalLoggerRegistry.getRegisteredLoggerNames()
}

// RegisterConfig atomically stores the current known logger config in the registry, and updates all registered loggers.
func RegisterConfig(logConfig []LoggerPatternConfig) error {
	return globalLoggerRegistry.registerConfig(logConfig)
}

// UpdateLoggerLevelWithCfg matches the desired logger to all patterns in the registry and updates its level.
func UpdateLoggerLevelWithCfg(name string) error {
	return globalLoggerRegistry.updateLoggerLevelWithCfg(name)
}

// GetCurrentConfig returns the logger config currently being used by the registry.
func GetCurrentConfig() []LoggerPatternConfig {
	return globalLoggerRegistry.getCurrentConfig()
}

// Consider removing this as a part of or after RSDK-8291 (https://viam.atlassian.net/browse/RSDK-8291)
// which may consolidate various calls to the rdk.networking logger into one.
func GetOrNewLogger(name string) (logger Logger) {
	globalLoggerRegistry.mu.Lock()
	defer globalLoggerRegistry.mu.Unlock()
	logger, ok := globalLoggerRegistry.loggers[name]
	if !ok {
		return NewLogger(name)
	}
	return logger
}
