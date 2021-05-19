package inject

import (
	"context"
	"image"

	"go.viam.com/core/camera"
	"go.viam.com/core/utils"
)

// Camera is an injected camera.
type Camera struct {
	camera.Camera
	NextFunc  func(ctx context.Context) (image.Image, func(), error)
	CloseFunc func() error
}

// Next calls the injected Next or the real version.
func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	if c.NextFunc == nil {
		return c.Camera.Next(ctx)
	}
	return c.NextFunc(ctx)
}

// Close calls the injected Close or the real version.
func (c *Camera) Close() error {
	if c.CloseFunc == nil {
		return utils.TryClose(c.Camera)
	}
	return c.CloseFunc()
}
