// Package register registers all relevant cameras and also subtype specific functions
package register

import (
	// for cameras.
	_ "go.viam.com/rdk/component/camera/fake"
	_ "go.viam.com/rdk/component/camera/ffmpeg"
	_ "go.viam.com/rdk/component/camera/imagesource"
	_ "go.viam.com/rdk/component/camera/imagetransform"
	_ "go.viam.com/rdk/component/camera/velodyne"
)
