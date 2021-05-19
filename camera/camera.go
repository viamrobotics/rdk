// Package camera defines a frame capturing device.
package camera

import (
	"fmt"

	"github.com/edaniels/gostream"

	"go.viam.com/core/rlog"
)

// A Camera represents anything that can capture frames.
type Camera interface {
	gostream.ImageSource

	// Reconfigure replaces this camera with the given camera.
	Reconfigure(newCamera Camera)
}

// ImageSource implements a Camera with a gostream.ImageSource.
type ImageSource struct {
	gostream.ImageSource
}

// Reconfigure replaces this camera with the given camera.
func (is *ImageSource) Reconfigure(newCamera Camera) {
	actual, ok := newCamera.(*ImageSource)
	if !ok {
		panic(fmt.Errorf("expected new camera to be %T but got %T", actual, newCamera))
	}
	if err := is.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	*is = *actual
}
