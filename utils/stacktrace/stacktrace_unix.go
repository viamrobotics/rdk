//go:build unix

package stacktrace

import (
	"os"
	"os/signal"
	"syscall"

	"go.viam.com/rdk/logging"
)

// stackTraceSignalHandler manages the SIGUSR1 signal handler for stack trace dumps.
type stackTraceSignalHandler struct {
	logger  logging.Logger
	sigChan chan os.Signal
	done    chan struct{}
}

// NewSignalHandler sets up a SIGUSR1 handler that dumps all goroutine
// stack traces when received. Returns a cleanup function that should be deferred.
func NewSignalHandler(logger logging.Logger) func() {
	handler := &stackTraceSignalHandler{
		logger:  logger,
		sigChan: make(chan os.Signal, 1),
		done:    make(chan struct{}),
	}
	signal.Notify(handler.sigChan, syscall.SIGUSR1)

	go func() {
		for {
			select {
			case <-handler.done:
				return
			case <-handler.sigChan:
				logger.Info("Received SIGUSR1, dumping stack traces")
				LogStackTrace(logger)
			}
		}
	}()

	cleanup := func() {
		signal.Stop(handler.sigChan)
		close(handler.done)
	}

	return cleanup
}
