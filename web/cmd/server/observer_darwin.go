//go:build darwin

package main

import (
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"

	"go.viam.com/rdk/logging"
)

// setupCameraObserver initializes the Darwin camera device observer for hot-plug support.
//
// On Darwin/macOS, SetupObserver must be called from the main thread (not a spawned
// goroutine) because AVFoundation requires that camera device notification events and Key-Value
// Observation (KVO) updates occur on the same thread as the producer. The mediadevices
// library uses runtime.LockOSThread() to pin the background goroutine to whatever thread
// calls SetupObserver, so calling it in main() ensures that we run on the correct thread.
//
// This is why SetupObserver is called here rather than in the webcam component constructor:
// component constructors can be invoked from arbitrary goroutines, which would violate
// AVFoundation's threading requirements. StartObserver (called in webcam.go) can safely run from
// any thread after SetupObserver has been called.
//
// See: https://github.com/pion/mediadevices/pull/670
func setupCameraObserver(logger logging.Logger) func() {
	if err := mediadevicescamera.SetupObserver(); err != nil {
		logger.Errorw("failed to set up darwin mediadevices camera observer", "error", err)
	}
	return func() {
		if err := mediadevicescamera.DestroyObserver(); err != nil {
			logger.Errorw("failed to destroy darwin mediadevices camera observer", "error", err)
		}
	}
}
