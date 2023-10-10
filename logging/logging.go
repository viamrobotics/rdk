// Package logging package contains functionality for viam-server logging.
package logging

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Level is an enum of log levels. Its value can be `DEBUG`, `INFO`, `WARN` or `ERROR`.
type Level int

const (
	// This numbering scheme serves two purposes:
	//   - A statement is logged if its log level matches or exceeds the configured level. I.e:
	//     Statement(WARN) >= LogConfig(INFO) would be logged because "1" > "0".
	//   - INFO is the default level. So we start counting at DEBUG=-1 such that INFO is given Go's
	//     zero-value.

	// DEBUG log level.
	DEBUG Level = iota - 1
	// INFO log level.
	INFO
	// WARN log level.
	WARN
	// ERROR log level.
	ERROR
)

func (level Level) String() string {
	switch level {
	case DEBUG:
		return "Debug"
	case INFO:
		return "Info"
	case WARN:
		return "Warn"
	case ERROR:
		return "Error"
	}

	panic(fmt.Sprintf("unreachable: %d", level))
}

// LevelFromString parses an input string to a log level. The string must be one of `debug`, `info`,
// `warn` or `error`. The parsing is case-insensitive. An error is returned if the input does not
// match one of labeled cases.
func LevelFromString(inp string) (Level, error) {
	switch strings.ToLower(inp) {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "warn":
		return WARN, nil
	case "error":
		return ERROR, nil
	}

	return DEBUG, fmt.Errorf("unknown log level: %q", inp)
}

// MarshalJSON converts a log level to a json string.
func (level Level) MarshalJSON() ([]byte, error) {
	return json.Marshal(level.String())
}

// UnmarshalJSON converts a json string to a log level.
func (level *Level) UnmarshalJSON(data []byte) (err error) {
	var levelStr string
	if err := json.Unmarshal(data, &levelStr); err != nil {
		return err
	}

	*level, err = LevelFromString(levelStr)
	return
}
