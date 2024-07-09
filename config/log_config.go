package config

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// LoggerPatternConfig is an instance of a level specification for a given logger.
type LoggerPatternConfig struct {
	Pattern string `json:"pattern"`
	Level   string `json:"level"`
}

const (
	validLoggerSectionName             = `[a-zA-Z0-9]+([_-]*[a-zA-Z0-9]+)*`
	validLoggerSectionNameWithWildcard = `(` + validLoggerSectionName + `|\*)`
	validLoggerSections                = validLoggerSectionNameWithWildcard + `(\.` + validLoggerSectionNameWithWildcard + `)*`
	validLoggerName                    = `^` + validLoggerSections + `$`
)

var loggerPatternRegexp = regexp.MustCompile(validLoggerName)

func validatePattern(pattern string) bool {
	return loggerPatternRegexp.MatchString(pattern)
}

// UpdateLoggerRegistry updates the logger registry if necessary  with the specified logConfig.
func UpdateLoggerRegistry(logConfig []LoggerPatternConfig, loggerRegistry map[string]logging.Logger) (map[string]logging.Logger, error) {
	newLogRegistry := make(map[string]logging.Logger)

	for _, lpc := range logConfig {
		if !validatePattern(lpc.Pattern) {
			return nil, errors.New("Failed to Validate a Pattern")
		}

		var matcher strings.Builder
		for idx, ch := range lpc.Pattern {
			switch ch {
			case '*':
				if idx == len(lpc.Pattern)-1 {
					matcher.WriteString(validLoggerSections)
				} else {
					matcher.WriteString(validLoggerSectionNameWithWildcard)
				}
			case '.':
				matcher.WriteString(`\.`)
			default:
				matcher.WriteRune(ch)
			}
		}
		r := regexp.MustCompile(matcher.String())

		for name, logger := range loggerRegistry {
			if _, ok := newLogRegistry[name]; !ok {
				newLogRegistry[name] = logger
			}
			if r.MatchString(name) {
				switch strings.ToLower(lpc.Level) {
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
