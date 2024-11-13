package app

import (
	common "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LogEntry holds the information of a single log entry.
type LogEntry struct {
	Host       string
	Level      string
	Time       *timestamppb.Timestamp
	LoggerName string
	Message    string
	Caller     *map[string]interface{}
	Stack      string
	Fields     []*map[string]interface{}
}

func logEntryFromProto(logEntry *common.LogEntry) *LogEntry {
	var fields []*map[string]interface{}
	for _, field := range logEntry.Fields {
		f := field.AsMap()
		fields = append(fields, &f)
	}
	caller := logEntry.Caller.AsMap()
	return &LogEntry{
		Host:       logEntry.Host,
		Level:      logEntry.Level,
		Time:       logEntry.Time,
		LoggerName: logEntry.LoggerName,
		Message:    logEntry.Message,
		Caller:     &caller,
		Stack:      logEntry.Stack,
		Fields:     fields,
	}
}
