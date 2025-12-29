//go:build darwin

package videosource

import (
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"

	"go.viam.com/rdk/logging"
)

// startCameraObserver starts the Darwin camera device observer for hot-plug support.
// This should be called after SetupObserver has been called from the main thread.
// See web/cmd/server/observer_darwin.go for details on the threading requirements.
func startCameraObserver(logger logging.Logger) {
	if err := mediadevicescamera.StartObserver(); err != nil {
		logger.Errorw("failed to start darwin mediadevices camera observer", "error", err)
	}
}
