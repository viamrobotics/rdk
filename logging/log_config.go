package logging

import (
	"regexp"
	"strings"
)

// LoggerPatternConfig is an instance of a level specification for a given logger.
type LoggerPatternConfig struct {
	Pattern string `json:"pattern"`
	Level   string `json:"level"`
}

const (
	// Pattern matching on loggers. Examples describe regular expression below.

	// e.g. "foo"
	validLoggerSectionName = `[a-zA-Z0-9]+([_-]*[a-zA-Z0-9]+)*`
	// e.g. "foo" or "*"
	validLoggerSectionNameWithWildcard = `(` + validLoggerSectionName + `|\*)`
	// e.g. "foo.bar"
	validLoggerSections = validLoggerSectionName + `(\.` + validLoggerSectionName + `)*`
	// e.g. "foo.*.foo"
	validLoggerSectionsWithWildcard = validLoggerSectionNameWithWildcard + `(\.` + validLoggerSectionNameWithWildcard + `)*`
	// Restricts above regex to be the entire pattern.
	validLoggerName = `^` + validLoggerSectionsWithWildcard + `$`

	// Resource configurations. Examples describe regular expression below.

	// e.g. "foo-bar"
	validNamespacePattern = `([\w-]+|\*)`
	// e.g. "service" or "component" or "*"
	validResourceTypePattern = `(service|component|\*)`
	// e.g. "foo-bar"
	validResourceSubTypePattern = validNamespacePattern
	// e.g. "foo-bar"
	validModelNamePattern = validNamespacePattern
	// e.g. "service:foo" or "remote:"
	validTypeSubsectionPattern = `(` + validResourceTypePattern + `:` + validResourceSubTypePattern + `|remote:)`
	// e.g. "rdk.resource_manager.rdk:component:motor/foo"
	validResourcePattern = `^rdk.resource_manager.` + validNamespacePattern + `:` + validTypeSubsectionPattern + `\/` +
		validModelNamePattern + `$` // e.g. "rdk.resource_manager.rdk:component:motor/foo"
)

var (
	loggerPatternRegexp   = regexp.MustCompile(validLoggerName)
	resourcePatternRegexp = regexp.MustCompile(validResourcePattern)
)

func validatePattern(pattern string) bool {
	return loggerPatternRegexp.MatchString(pattern) || resourcePatternRegexp.MatchString(pattern)
}

func buildRegexFromPattern(pattern string) string {
	var matcher strings.Builder
	matcher.WriteRune('^')
	for _, ch := range pattern {
		switch ch {
		case '*':
			matcher.WriteString(`.*`)
		case '.':
			matcher.WriteString(`\.`)
		default:
			matcher.WriteRune(ch)
		}
	}
	matcher.WriteRune('$')
	return matcher.String()
}
