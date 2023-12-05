//go:build linux

// This package creates two virtual cameras and streams test video to them until the program
// halts (with ctrl-c for example). These cameras are discoverable by app.
//
// Prerequisites: run etc/v4l2loopback_setup.sh which will install GStreamer and V4L2Loopback.
package main

import (
	"os"
	"os/signal"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/testutils/vcamera"
)

func main() {
	logger := logging.NewDebugLogger("vcamera")
	config, err := vcamera.Builder(logger).
		NewCamera(1, "Low-res Camera", vcamera.Resolution{Width: 640, Height: 480}).
		NewCamera(2, "Hi-res Camera", vcamera.Resolution{Width: 1280, Height: 720}).
		Stream()

	defer func() {
		if err := config.Shutdown(); err != nil {
			logger.Fatal(err)
		}
	}()

	if err != nil {
		logger.Fatal(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	logger.Info("Streaming on /dev/video1 and /dev/video2...")
	// wait for ctrl-c or other interrupt
	<-c
	logger.Info("Finished streaming.")
}
