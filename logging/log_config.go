package logging

import (
	"errors"
	"regexp"
	"strings"
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

func (lr *loggerRegistry) updateLoggerRegistry(logConfig []LoggerPatternConfig) error {
	for _, lpc := range logConfig {
		if !validatePattern(lpc.Pattern) {
			return errors.New("failed to validate a pattern")
		}

		var matcher strings.Builder
		matcher.WriteRune('^')
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
		matcher.WriteRune('$')
		r, err := regexp.Compile(matcher.String())
		if err != nil {
			return err
		}

		for _, name := range lr.getRegisteredLoggerNames() {
			if r.MatchString(name) {
				level, err := LevelFromString(lpc.Level)
				if err != nil {
					return err
				}
				err = lr.updateLoggerLevel(name, level)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// UpdateLoggerRegistry updates the logger registry if necessary  with the specified logConfig.
func UpdateLoggerRegistry(logConfig []LoggerPatternConfig) error {
	return loggerManager.updateLoggerRegistry(logConfig)
}
