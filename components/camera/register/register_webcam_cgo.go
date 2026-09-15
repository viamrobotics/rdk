//go:build (windows && cgo) || viam_windows_resource_dump

// Package register registers all relevant cameras and also API specific functions
package register

// Package register registers the webcam driver on Windows builds that have cgo enabled,
// independent of the no_cgo tag so other cgo-gated features (graphviz, nlopt, x264
// streaming, etc.) keep using their existing no_cgo stubs on Windows.
import (
	// for cameras.
	_ "go.viam.com/rdk/components/camera/videosource"
)
