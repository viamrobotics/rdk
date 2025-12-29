// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"runtime"

	"go.viam.com/utils"

	// registers all components.
	_ "go.viam.com/rdk/components/arm/wrapper" // this is special
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"

	// registers all services.
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/web/server"

	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
)

var logger = logging.NewDebugLogger("entrypoint")

func main() {
	// mediadevices camera observer is for supporting webcam hot unplug/replug on darwin.
	// MacOS does not support poll-based querying for connected devices, and requires a dedicated
	// goroutine to monitor for connect/disconnect events.
	//
	// On Darwin/macOS, SetupObserver must be called from the main thread (not a spawned
	// goroutine) because AVFoundation requires that camera device notification events and Key-Value
	// Observation (KVO) updates occur on the same thread as the producer. The mediadevices
	// library uses runtime.LockOSThread() to pin the background goroutine to whatever thread
	// calls SetupObserver, so calling it here in main() ensures that we run on the correct thread.
	//
	// This is why SetupObserver is called here rather than in the webcam component constructor:
	// component constructors can be invoked from arbitrary goroutines, which would violate
	// AVFoundation's threading requirements. StartObserver (called in webcam.go) can safely run from
	// any thread after SetupObserver has been called.
	//
	// See: https://github.com/pion/mediadevices/pull/670
	if runtime.GOOS == "darwin" {
		if err := mediadevicescamera.SetupObserver(); err != nil {
			logger.Errorw("failed to set up darwin mediadevices camera observer", "error", err)
		}
		defer func() {
			if err := mediadevicescamera.DestroyObserver(); err != nil {
				logger.Errorw("failed to destroy darwin mediadevices camera observer", "error", err)
			}
		}()
	}

	utils.ContextualMain(server.RunServer, logger)
}
