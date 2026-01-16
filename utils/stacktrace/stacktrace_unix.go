//go:build unix

package stacktrace

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.viam.com/rdk/logging"
)

// stackTraceSignalHandler manages the SIGUSR1 signal handler for stack trace dumps.
type stackTraceSignalHandler struct {
	logger   logging.Logger
	sigChan  chan os.Signal
	done     chan struct{}
	mu       sync.Mutex
	callback func()
}

// NewSignalHandler sets up a SIGUSR1 handler that dumps all goroutine
// stack traces when received. Returns a handler that can be used to register
// additional callbacks and a cleanup function that should be deferred.
func NewSignalHandler(logger logging.Logger) (*stackTraceSignalHandler, func()) {
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

				// Forward to modules if callback is registered
				handler.mu.Lock()
				cb := handler.callback
				handler.mu.Unlock()
				if cb != nil {
					cb()
				}
			}
		}
	}()

	cleanup := func() {
		signal.Stop(handler.sigChan)
		close(handler.done)
	}

	return handler, cleanup
}

// SetCallback sets a callback function that will be called when SIGUSR1
// is received, after dumping viam-server's stack traces. This is typically used
// to forward the signal to module processes.
func (h *stackTraceSignalHandler) SetCallback(cb func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callback = cb
}
