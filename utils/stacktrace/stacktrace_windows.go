//go:build windows

package stacktrace

import "go.viam.com/rdk/logging"

// stackTraceSignalHandler is a no-op on Windows since SIGUSR1 doesn't exist.
type stackTraceSignalHandler struct{}

// NewSignalHandler is a no-op on Windows since SIGUSR1 doesn't exist.
func NewSignalHandler(logger logging.Logger) (*stackTraceSignalHandler, func()) {
	return &stackTraceSignalHandler{}, func() {}
}

// SetCallback is a no-op on Windows since SIGUSR1 doesn't exist.
func (h *stackTraceSignalHandler) SetCallback(cb func()) {}
