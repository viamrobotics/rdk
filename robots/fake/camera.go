package fake

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"

	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"

	"github.com/edaniels/gostream"
)

func init() {
	registry.RegisterCamera("fake", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		return &Camera{Name: config.Name}, nil
	})
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	Name string
}

// Next always returns the same image with a red dot in the center.
func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	img.Set(16, 16, rimage.Red)
	return img, func() {}, nil
}

// Reconfigure replaces this camera with the given camera.
func (c *Camera) Reconfigure(newCamera camera.Camera) {
	actual, ok := newCamera.(*Camera)
	if !ok {
		panic(fmt.Errorf("expected new camera to be %T but got %T", actual, newCamera))
	}
	if err := c.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	*c = *actual
}

// Close does nothing.
func (c *Camera) Close() error {
	return nil
}
