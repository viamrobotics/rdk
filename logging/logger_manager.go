package logging

import (
	"fmt"
	"sync"
)

type loggerRegistry struct {
	mu      sync.RWMutex
	loggers map[string]Logger
	names   map[Logger]string
}

var loggerManager = newLoggerManager()

func newLoggerManager() *loggerRegistry {
	return &loggerRegistry{
		loggers: make(map[string]Logger),
		names:   make(map[Logger]string),
	}
}

func (lr *loggerRegistry) registerLogger(name string, logger Logger) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.loggers[name] = logger
	lr.names[logger] = name
}

func (lr *loggerRegistry) nameOf(logger Logger) (name string, ok bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	name, ok = lr.names[logger]
	return name, ok
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
