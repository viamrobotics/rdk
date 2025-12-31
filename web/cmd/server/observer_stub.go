//go:build !darwin

package main

import "go.viam.com/rdk/logging"

// setupCameraObserver is a no-op on non-Darwin platforms.
// Camera hot-plug detection via AVFoundation observers is only supported on macOS.
// On Linux, poll-based device enumeration is used instead (checking /dev/video* files).
// On Windows, Media Foundation APIs handle device enumeration differently.
func setupCameraObserver(_ logging.Logger) func() {
	return func() {}
}
