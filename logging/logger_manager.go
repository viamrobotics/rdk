package logging

import (
	"sync"
)

type LoggerRegistry struct {
	mu      sync.Mutex
	loggers map[string]Logger
	names   map[Logger]string
}

var loggerManager = NewLoggerManager()

func NewLoggerManager() *LoggerRegistry {
	return &LoggerRegistry{
		loggers: make(map[string]Logger),
		names:   make(map[Logger]string),
	}
}

func (lr *LoggerRegistry) RegisterLogger(name string, logger Logger) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.loggers[name] = logger
	lr.names[logger] = name
}

func (lr *LoggerRegistry) NameOf(logger Logger) (name string, ok bool) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	name, ok = lr.names[logger]
	return name, ok
}

func (lr *LoggerRegistry) UpdateLoggerLevel(name string, level Level) {
	lr.loggers[name].SetLevel(level)
}
