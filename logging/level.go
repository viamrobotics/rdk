package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

// Level is an enum of log levels. Its value can be `DEBUG`, `INFO`, `WARN` or `ERROR`.
type Level int

// AtomicLevel is a level that can be concurrently accessed.
type AtomicLevel struct {
	val *atomic.Int32
}

// NewAtomicLevelAt creates a new AtomicLevel at the input `initLevel`.
func NewAtomicLevelAt(initLevel Level) AtomicLevel {
	ret := AtomicLevel{
		new(atomic.Int32),
	}
	ret.Set(initLevel)
	return ret
}

// Set changes the level.
func (level AtomicLevel) Set(newLevel Level) {
	level.val.Store(int32(newLevel))
}

// Get returns the level.
func (level AtomicLevel) Get() Level {
	return Level(level.val.Load())
}

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

// AsZap converts the Level to a `zapcore.Level`.
func (level Level) AsZap() zapcore.Level {
	switch level {
	case DEBUG:
		return zapcore.DebugLevel
	case INFO:
		return zapcore.InfoLevel
	case WARN:
		return zapcore.WarnLevel
	case ERROR:
		return zapcore.ErrorLevel
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
