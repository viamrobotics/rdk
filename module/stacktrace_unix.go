//go:build unix

package module

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"go.viam.com/rdk/logging"
)

// setupStackTraceSignalHandler sets up a SIGUSR1 handler that dumps all goroutine
// stack traces when received. This is useful for debugging when the agent restarts
// viam-server and viam-server forwards SIGUSR1 to modules.
// Returns a cleanup function that should be deferred.
func setupStackTraceSignalHandler(logger logging.Logger) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-sigChan:
				logger.Info("Received SIGUSR1, dumping stack traces")
				logStackTrace(logger)
			}
		}
	}()

	return func() {
		signal.Stop(sigChan)
		close(done)
	}
}

// logStackTrace dumps all goroutine stack traces to the logger.
func logStackTrace(logger logging.Logger) {
	bufSize := 1 << 20
	traces := make([]byte, bufSize)
	traceSize := runtime.Stack(traces, true)
	message := "module stack trace dump"
	if traceSize == bufSize {
		message = fmt.Sprintf("%s (warning: backtrace truncated to %v bytes)", message, bufSize)
	}
	logger.Infof("%s:\n%s", message, traces[:traceSize])
}
