// Package camera defines a frame capturing device.
package camera

import (
	"github.com/edaniels/gostream"
)

// A Camera represents anything that can capture frames.
type Camera interface {
	gostream.ImageSource
}

// ImageSource implements a Camera with a gostream.ImageSource.
type ImageSource struct {
	gostream.ImageSource
}
