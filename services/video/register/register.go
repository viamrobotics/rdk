// Package register registers the video service and its implementations.
package register

import (
	// register video.
	_ "go.viam.com/rdk/services/video"
	_ "go.viam.com/rdk/services/video/fake"
)
