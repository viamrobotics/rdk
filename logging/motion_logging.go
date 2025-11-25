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

	// Default viam-server logging. `*.mp` is at its default value (presumably INFO), but loggers
	// underneath `mp` are chatty. Set them to only emit WARN+ logs.
	if !testing.Testing() {
		registry.Update([]LoggerPatternConfig{
			{
				Pattern: "*.mp.*",
				Level:   "WARN",
			},
		}, warnLogger)
		return
	}

	// We are in testing. If the IK debug env variable is also present, set all motion planning to
	// DEBUG.
	if debugIkMinFunc {
		registry.Update([]LoggerPatternConfig{
			// This targets both `*.mp` and `*.mp.*`.
			{
				Pattern: "*.mp*",
				Level:   "DEBUG",
			},
		}, warnLogger)
	} else {
		registry.Update([]LoggerPatternConfig{
			// `mp` at DEBUG is reasonable, everything under `mp` is chatty and set to only emit
			// INFO+ for testing.
			{
				Pattern: "*.mp",
				Level:   "DEBUG",
			},
			{
				Pattern: "*.mp.*",
				Level:   "INFO",
			},
		}, warnLogger)
	}
}
