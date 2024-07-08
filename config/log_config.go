package config

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// LogPatternConfig is an instance of a level specification for a given logger.
type LogPatternConfig struct {
	Pattern string `json:"pattern"`
	Level   string `json:"level"`
}

func validatePattern(pattern string) bool {
	size := len(pattern)
	if size == 0 || pattern[0] == '.' || pattern[size-1] == '.' {
		return false
	}
	for i := 1; i < size; i++ {
		if pattern[i] == '.' && pattern[i-1] == '.' {
			return false
		}
	}
	r := regexp.MustCompile(`^[a-z_-]+$|^\*$`)
	for _, sep := range strings.Split(pattern, ".") {
		if !r.MatchString(sep) {
			return false
		}
	}
	return true
}

// UpdateLoggerRegistry updates the logger registry if necessary  with the specified logConfig.
func UpdateLoggerRegistry(logConfig []LogPatternConfig, loggerRegistry map[string]logging.Logger) (map[string]logging.Logger, error) {
	newLogRegistry := make(map[string]logging.Logger)

	for _, lpc := range logConfig {
		if !validatePattern(lpc.Pattern) {
			return nil, errors.New("Failed to Validate a Pattern")
		}

		matcher := "^"
		for _, ch := range lpc.Pattern {
			if ch == '*' {
				matcher += `(\w|\.)+`
			} else {
				matcher += string(ch)
			}
		}
		matcher += "$"
		r := regexp.MustCompile(matcher)

		for name, logger := range loggerRegistry {
			if _, ok := newLogRegistry[name]; !ok {
				newLogRegistry[name] = logger
			}
			if r.MatchString(name) {
				switch lowercaseLevel := strings.ToLower(lpc.Level); lowercaseLevel {
				case "debug":
					newLogRegistry[name].SetLevel(logging.DEBUG)
				case "info":
					newLogRegistry[name].SetLevel(logging.INFO)
				case "warn":
					newLogRegistry[name].SetLevel(logging.WARN)
				case "error":
					newLogRegistry[name].SetLevel(logging.ERROR)
				}
			}
		}
	}

	return newLogRegistry, nil
}
