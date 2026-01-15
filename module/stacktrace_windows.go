//go:build windows

package module

import "go.viam.com/rdk/logging"

// setupStackTraceSignalHandler is a no-op on Windows since SIGUSR1 doesn't exist.
func setupStackTraceSignalHandler(logger logging.Logger) func() {
	return func() {}
}
