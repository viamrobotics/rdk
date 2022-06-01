package inject

import (
	"context"
	"image"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/pointcloud"
)

// Camera is an injected camera.
type Camera struct {
	camera.Camera
	DoFunc             func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	NextFunc           func(ctx context.Context) (image.Image, func(), error)
	NextPointCloudFunc func(ctx context.Context) (pointcloud.PointCloud, error)
	CloseFunc          func(ctx context.Context) error
}

// Next calls the injected Next or the real version.
func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	if c.NextFunc == nil {
		return c.Camera.Next(ctx)
	}
	return c.NextFunc(ctx)
}

// NextPointCloud calls the injected NextPointCloud or the real version.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c.NextPointCloudFunc == nil {
		return c.Camera.NextPointCloud(ctx)
	}
	return c.NextPointCloudFunc(ctx)
}

// Close calls the injected Close or the real version.
func (c *Camera) Close(ctx context.Context) error {
	if c.CloseFunc == nil {
		return utils.TryClose(ctx, c.Camera)
	}
	return c.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (c *Camera) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if c.DoFunc == nil {
		return c.Camera.Do(ctx, cmd)
	}
	return c.DoFunc(ctx, cmd)
}
