package logging

import (
	"fmt"
	"sync"
)

type loggerRegistry struct {
	mu      sync.RWMutex
	loggers map[string]Logger
}

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
	return logger, ok
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

// Exported Functions specifically for use on global logger manager
func RegisterLogger(name string, logger Logger) {
	loggerManager.mu.Lock()
	defer loggerManager.mu.Unlock()
	loggerManager.loggers[name] = logger
}

func LoggerNamed(name string) (logger Logger, ok bool) {
	loggerManager.mu.RLock()
	defer loggerManager.mu.RUnlock()
	logger, ok = loggerManager.loggers[name]
	return logger, ok
}

func UpdateLoggerLevel(name string, level Level) error {
	loggerManager.mu.RLock()
	defer loggerManager.mu.RUnlock()
	logger, ok := loggerManager.loggers[name]
	if !ok {
		return fmt.Errorf("logger named %s not recognized", name)
	}
	logger.SetLevel(level)
	return nil
}
