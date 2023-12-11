package module

import (
	"go.viam.com/rdk/logging"
	"os"

	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type moduleAppender struct {
	stdoutAppender *logging.ConsoleAppender

	// If Module is set, moduleAppender sends log events to the Unix socket at
	// Module.parentAddr via gRPC. Otherwise, moduleAppender logs to STDOUT.
	module *Module
}

func newModuleAppender() *moduleAppender {
	return &moduleAppender{}
}

func (ma *moduleAppender) setModule(m *Module) {
	ma.module = m
}

// Write sends the log entry back to the module's parent via gRPC or, if not
// possible, outputs the log entry to the underlying stream.
func (ma *moduleAppender) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if ma.module == nil {
		return ma.stdoutAppender.Write(entry, fields)
	}

	log := &pb.LogEntry{
		Host:       ma.module.addr, // TODO(benji)?
		Level:      entry.Level.String(),
		Time:       timestamppb.New(entry.Time),
		LoggerName: entry.LoggerName,
		Message:    entry.Message,
		Stack:      entry.Stack,
	}

	// TODO(benji): send over gRPC.

	return nil
}

// Sync is a no-op (moduleAppenders do not currently have buffers that needs
// flushing via Sync).
func (ma *moduleAppender) Sync() error {
	return nil
}

type moduleLogger struct {
	logging.Logger
	modAppender *moduleAppender
}

// NewLoggerFromArgs can be used to create a logging.Logger at "DebugLevel" if
// "--log-level=debug" is the third argument in os.Args and at "InfoLevel"
// otherwise. See config.Module.LogLevel documentation for more info on how
// to start modules with a "log-level" commandline argument. The created logger
// will send log events back to the module's parent (the RDK) via gRPC when
// possible and to STDOUT when not possible.
func NewLoggerFromArgs(moduleName string) logging.Logger {
	modAppender := newModuleAppender()
	baseLogger := logging.NewBlankLogger(moduleName)
	baseLogger.AddAppender(modAppender)

	// Use DEBUG logging only if 3rd OS argument is "--log-level=debug".
	if len(os.Args) < 3 || os.Args[2] != "--log-level=debug" {
		baseLogger.SetLevel(logging.INFO)
	}
	return &moduleLogger{baseLogger, modAppender}
}

// startLoggingViaGRPC switches the moduleLogger to gRPC logging. To be used
// once a ReadyRequest has been received from the parent, and the module's
// parent's address is known.
func (ml *moduleLogger) startLoggingViaGRPC(m *Module) {
	ml.modAppender.setModule(m)
}
