package logging

import (
	"fmt"
	"sync"
)

type loggerRegistry struct {
	mu      sync.RWMutex
	loggers map[string]Logger
}

// TODO(RSDK-8250): convert loggerManager from global variable to variable on local robot.
var loggerManager = newLoggerManager()

func newLoggerManager() *loggerRegistry {
	return &loggerRegistry{
		loggers: make(map[string]Logger),
	}
}

func (lr *loggerRegistry) registerLogger(name string, logger Logger) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.loggers[name] = logger
}

func (lr *loggerRegistry) loggerNamed(name string) (logger Logger, ok bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	logger, ok = lr.loggers[name]
	return
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

func (lr *loggerRegistry) getRegisteredLoggerNames() []string {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	registeredNames := make([]string, 0, len(loggerManager.loggers))
	for name := range lr.loggers {
		registeredNames = append(registeredNames, name)
	}
	return registeredNames
}

// Exported Functions specifically for use on global logger manager.

// RegisterLogger registers a new logger with a given name.
func RegisterLogger(name string, logger Logger) {
	loggerManager.registerLogger(name, logger)
}

// LoggerNamed returns logger with specified name if exists.
func LoggerNamed(name string) (logger Logger, ok bool) {
	return loggerManager.loggerNamed(name)
}

// UpdateLoggerLevel assigns level to appropriate logger in the registry.
func UpdateLoggerLevel(name string, level Level) error {
	return loggerManager.updateLoggerLevel(name, level)
}

// GetRegisteredLoggerNames returns the names of all loggers in the registry.
func GetRegisteredLoggerNames() []string {
	return loggerManager.getRegisteredLoggerNames()
}
