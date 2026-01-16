// Package stacktrace provides utilities for dumping goroutine stack traces
// for debugging purposes
package stacktrace

import (
	"context"
	"fmt"
	"runtime"

	"go.viam.com/rdk/logging"
)

// LogStackTrace dumps all goroutine stack traces to the logger.
// "rdk.stack_traces" is listed as a diagnostic logger in app; users will not see
// viam-server stack traces by default on app.viam.com.
func LogStackTrace(logger logging.Logger) {
	logger = logger.Sublogger("stack_traces")
	bufSize := 1 << 20
	traces := make([]byte, bufSize)
	traceSize := runtime.Stack(traces, true)
	message := "backtrace at robot shutdown"
	if traceSize == bufSize {
		message = fmt.Sprintf("%s (warning: backtrace truncated to %v bytes)", message, bufSize)
	}
	logger.Infof("%s, %s", message, traces[:traceSize])
}

// LogStackTraceAndCancel dumps stack traces and then invokes the cancel function.
// This is useful for signal handlers that need to both log and shutdown.
func LogStackTraceAndCancel(cancel context.CancelFunc, logger logging.Logger) {
	LogStackTrace(logger)
	cancel()
}
