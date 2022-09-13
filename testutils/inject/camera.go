package inject

import (
	"context"

	"github.com/edaniels/gostream"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

// Camera is an injected camera.
type Camera struct {
	camera.Camera
	DoFunc     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StreamFunc func(
		ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.VideoStream, error)
	NextPointCloudFunc func(ctx context.Context) (pointcloud.PointCloud, error)
	ProjectorFunc      func(ctx context.Context) (rimage.Projector, error)
	GetPropertiesFunc  func(ctx context.Context) (camera.Properties, error)
	CloseFunc          func(ctx context.Context) error
}

// NextPointCloud calls the injected NextPointCloud or the real version.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c.NextPointCloudFunc == nil {
		return c.Camera.NextPointCloud(ctx)
	}
	return c.NextPointCloudFunc(ctx)
}

// Stream calls the injected Stream or the real version.
func (c *Camera) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	if c.StreamFunc == nil {
		return c.Camera.Stream(ctx, errHandlers...)
	}
	return c.StreamFunc(ctx, errHandlers...)
}

// Projector calls the injected Projector or the real version.
func (c *Camera) Projector(ctx context.Context) (rimage.Projector, error) {
	if c.ProjectorFunc == nil {
		return c.Camera.Projector(ctx)
	}
	return c.ProjectorFunc(ctx)
}

// GetProperties calls the injected GetProperties or the real version.
func (c *Camera) GetProperties(ctx context.Context) (camera.Properties, error) {
	if c.GetPropertiesFunc == nil {
		return c.Camera.GetProperties(ctx)
	}
	return c.GetPropertiesFunc(ctx)
}

// Close calls the injected Close or the real version.
func (c *Camera) Close(ctx context.Context) error {
	if c.CloseFunc == nil {
		return utils.TryClose(ctx, c.Camera)
	}
	return c.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (c *Camera) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if c.DoFunc == nil {
		return c.Camera.DoCommand(ctx, cmd)
	}
	return c.DoFunc(ctx, cmd)
}
