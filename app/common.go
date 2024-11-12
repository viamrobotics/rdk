package app

import (
	common "go.viam.com/api/common/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LogEntry struct {
	Host string
	Level string
	Time *timestamppb.Timestamp
	LoggerName string
	Message string
	Caller *map[string]interface{}
	Stack string
	Fields []*map[string]interface{}
}

func ProtoToLogEntry(logEntry *common.LogEntry) (*LogEntry, error) {
	var fields []*map[string]interface{}
	for _, field := range(logEntry.Fields) {
		f := field.AsMap()
		fields = append(fields, &f)
	}
	caller := logEntry.Caller.AsMap()
	return &LogEntry{
		Host: logEntry.Host,
		Level: logEntry.Level,
		Time: logEntry.Time,
		LoggerName: logEntry.LoggerName,
		Message: logEntry.Message,
		Caller: &caller,
		Stack: logEntry.Stack,
		Fields: fields,
	}, nil
}

func LogEntryToProto(logEntry *LogEntry) (*common.LogEntry, error) {
	var fields []*structpb.Struct
	for _, field := range(logEntry.Fields) {
		f, err := protoutils.StructToStructPb(field)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	caller, err := protoutils.StructToStructPb(logEntry.Caller)
	if err != nil {
		return nil, err
	}
	return &common.LogEntry{
		Host: logEntry.Host,
		Level: logEntry.Level,
		Time: logEntry.Time,
		LoggerName: logEntry.LoggerName,
		Message: logEntry.Message,
		Caller: caller,
		Stack: logEntry.Stack,
		Fields: fields,
	}, nil
}
