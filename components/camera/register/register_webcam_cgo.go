//go:build (windows && cgo) || viam_windows_resource_dump

// Package register registers all relevant cameras and also API specific functions
// This registers the webcam driver on Windows builds that have cgo enabled
package register

import (
	// for cameras.
	_ "go.viam.com/rdk/components/camera/videosource"
)
