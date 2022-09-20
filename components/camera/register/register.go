// Package register registers all relevant cameras and also subtype specific functions
package register

import (
	// for cameras.
	_ "go.viam.com/rdk/components/camera/fake"
	_ "go.viam.com/rdk/components/camera/ffmpeg"
	_ "go.viam.com/rdk/components/camera/transformpipeline"
	_ "go.viam.com/rdk/components/camera/velodyne"
	_ "go.viam.com/rdk/components/camera/videosource"
)
