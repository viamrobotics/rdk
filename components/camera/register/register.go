//go:build !no_media

// Package register registers all relevant cameras and also API specific functions
package register

import (
	// for cameras.
	_ "go.viam.com/rdk/components/camera/align"
	_ "go.viam.com/rdk/components/camera/fake"
	_ "go.viam.com/rdk/components/camera/ffmpeg"
	_ "go.viam.com/rdk/components/camera/replaypcd"
	_ "go.viam.com/rdk/components/camera/rtsp"
	_ "go.viam.com/rdk/components/camera/transformpipeline"
	_ "go.viam.com/rdk/components/camera/ultrasonic"
	_ "go.viam.com/rdk/components/camera/velodyne"
	_ "go.viam.com/rdk/components/camera/videosource"
)
