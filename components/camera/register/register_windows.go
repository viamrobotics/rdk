//go:build windows && cgo

// Package register registers the webcam driver on Windows builds that have cgo enabled,
// independent of the no_cgo tag so other cgo-gated features (graphviz, nlopt, x264
// streaming, etc.) keep using their existing no_cgo stubs on Windows.
package register

import (
	// for cameras.
	_ "go.viam.com/rdk/components/camera/videosource"
)
