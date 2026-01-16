package stacktrace

import (
	"context"
	"fmt"
	"runtime"

	"go.viam.com/rdk/logging"
)

// logStackTrace dumps all goroutine stack traces to the logger.
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

func LogStackTraceAndCancel(cancel context.CancelFunc, logger logging.Logger) {
	LogStackTrace(logger)
	cancel()
}
