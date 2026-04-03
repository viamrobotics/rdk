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

	mpPatterns := []LoggerPatternConfig{}

	// We are in testing. If the IK debug env variable is also present, set all motion planning to
	// DEBUG.
	if debugIkMinFunc {
		mpPatterns = append(mpPatterns, LoggerPatternConfig{
			// This targets both `*.mp` and `*.mp.*`.
			Pattern: "*.mp*",
			Level:   "DEBUG",
		})
	} else {
		mpPatterns = append(mpPatterns,
			// `mp` at DEBUG is reasonable, everything under `mp` is chatty and set to only emit
			// INFO+ for testing.
			LoggerPatternConfig{
				Pattern: "*.mp",
				Level:   "DEBUG",
			},
			LoggerPatternConfig{
				Pattern: "*.mp.*",
				Level:   "INFO",
			},
		)
	}

	// The `startup-profile` logger is used for some startup motion plan requests. Avoid logging
	// those the progress of those requests. Assume if there's a computational problem, there's
	// better suited motion planning test that will surface appropriate details.
	mpPatterns = append(mpPatterns, LoggerPatternConfig{
		Pattern: "startup-profile.*",
		Level:   "WARN",
	})

	registry.Update(mpPatterns, warnLogger)
}
