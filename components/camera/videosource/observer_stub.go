//go:build !darwin

package videosource

import "go.viam.com/rdk/logging"

// startCameraObserver is a no-op on non-Darwin platforms.
// Camera hot-plug detection via AVFoundation observers is only supported on macOS.
func startCameraObserver(_ logging.Logger) {}
