package module

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap/zapcore"

	"go.viam.com/rdk/logging"
)

type moduleAppender struct {
	stdoutAppender *logging.ConsoleAppender

	// If Module is set, moduleAppender sends log events to the Unix socket at
	// Module.parentAddr via gRPC. Otherwise, moduleAppender logs to STDOUT.
	module *Module
}

func newModuleAppender() *moduleAppender {
	stdoutAppender := logging.NewStdoutAppender()
	return &moduleAppender{stdoutAppender: &stdoutAppender}
}

func (ma *moduleAppender) setModule(m *Module) {
	ma.module = m
}

// Write sends the log entry back to the module's parent via gRPC or, if not
// possible, outputs the log entry to the underlying stream.
func (ma *moduleAppender) Write(log zapcore.Entry, fields []zapcore.Field) error {
	if ma.module == nil {
		return ma.stdoutAppender.Write(log, fields)
	}

	// Only give 5 seconds for ModuleLog call in case parent (RDK) is shutting
	// down or otherwise unreachable.
	moduleLogCtx, moduleLogCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer moduleLogCancel()
	return ma.module.parent.Log(moduleLogCtx, log, fields)
}

// Sync is a no-op (moduleAppenders do not currently have buffers that needs
// flushing via Sync).
func (ma *moduleAppender) Sync() error {
	return nil
}

// TODO(RSDK-6280): Preserve timezones for moduleLogger.
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
//
// `moduleName` will be the name of the created logger. Pass `""` if you wish to
// use the value specified by the `VIAM_MODULE_NAME` environment variable.
func NewLoggerFromArgs(moduleName string) logging.Logger {
	// If no `moduleName` was specified, grab it from the environment (will still be empty
	// string if not specified in environment.)
	if moduleName == "" {
		moduleName, _ = os.LookupEnv("VIAM_MODULE_NAME")
	}

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
