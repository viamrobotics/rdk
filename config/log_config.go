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
	validLoggerSections                = validLoggerSectionName + `(\.` + validLoggerSectionName + `)*`
	validLoggerSectionsWithWildcard    = validLoggerSectionNameWithWildcard + `(\.` + validLoggerSectionNameWithWildcard + `)*`
	validLoggerName                    = `^` + validLoggerSectionsWithWildcard + `$`
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
			return nil, errors.New("failed to validate a pattern")
		}

		var matcher strings.Builder
		for _, ch := range lpc.Pattern {
			switch ch {
			case '*':
				matcher.WriteString(validLoggerSections)
			case '.':
				matcher.WriteString(`\.`)
			default:
				matcher.WriteRune(ch)
			}
		}
		r, err := regexp.Compile(matcher.String())
		if err != nil {
			return nil, err
		}

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
