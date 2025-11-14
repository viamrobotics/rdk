package logging

import (
	"os"
	"testing"
)

// GetenvBool is copied from the utils directory. The utils package sillily depends on logging,
// hence would create an import cycle if we re-used that one.
func GetenvBool(v string, def bool) bool {
	x := os.Getenv(v)
	if x == "" {
		return def
	}

	return x[0] == 't' || x[0] == 'T' || x[0] == '1'
}

var debugIkMinFunc = GetenvBool("DEBUG_IK_MINFUNC", false)

func applyMotionRegistryOptions(registry *Registry) {
	warnLogger := &impl{
		name:                 "startup",
		level:                NewAtomicLevelAt(DEBUG),
		appenders:            []Appender{NewStdoutAppender()},
		recentMessageCounts:  make(map[string]int),
		recentMessageEntries: make(map[string]LogEntry),
	}

	if !testing.Testing() {
		registry.Update([]LoggerPatternConfig{
			{
				Pattern: "*.ik",
				Level:   "WARN",
			},
			{
				Pattern: "*.cbirrt",
				Level:   "WARN",
			},
		}, warnLogger)
		return
	}

	if debugIkMinFunc {
		registry.Update([]LoggerPatternConfig{
			{
				Pattern: "*.ik",
				Level:   "DEBUG",
			},
			{
				Pattern: "*.cbirrt",
				Level:   "DEBUG",
			},
		}, warnLogger)
	} else {
		registry.Update([]LoggerPatternConfig{
			{
				Pattern: "*.ik",
				Level:   "INFO",
			},
			{
				Pattern: "*.cbirrt",
				Level:   "INFO",
			},
		}, warnLogger)
	}
}
