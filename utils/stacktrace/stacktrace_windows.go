//go:build windows

package stacktrace

import "go.viam.com/rdk/logging"

// stackTraceSignalHandler is a no-op on Windows since signals don't exist.
type stackTraceSignalHandler struct{}

// NewSignalHandler is a no-op on Windows since signals don't exist.
func NewSignalHandler(logger logging.Logger) func() {
	return func() {}
}
