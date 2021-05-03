package fake

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/gostream"
)

func init() {
	api.RegisterCamera("fake", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		return &Camera{Name: config.Name}, nil
	})
}

type Camera struct {
	Name string
}

func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	img.Set(16, 16, rimage.Red)
	return img, func() {}, nil
}

func (c *Camera) Close() error {
	return nil
}
