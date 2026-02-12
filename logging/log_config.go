package logging

import (
	"strings"
)

// LoggerPatternConfig is an instance of a level specification for a given logger.
type LoggerPatternConfig struct {
	Pattern string `json:"pattern"`
	Level   string `json:"level"`
}

// BuildRegexFromPattern creates a compilable regex from a log pattern.
func BuildRegexFromPattern(pattern string) string {
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
