package camera

import (
	"context"
	"sync"

	"github.com/edaniels/gostream"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage/transform"
)

// WaitGroupCamera is a wrapper for camera.Camera with a sync.WaitGroup.
type WaitGroupCamera struct {
	Cam                     Camera
	ActiveBackgroundWorkers sync.WaitGroup
}

// DoCommand wraps camera.Camera.DoCommand.
func (c *WaitGroupCamera) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return c.Cam.DoCommand(ctx, cmd)
}

// Projector wraps camera.Camera.Projector.
func (c *WaitGroupCamera) Projector(ctx context.Context) (transform.Projector, error) {
	return c.Cam.Projector(ctx)
}

// Stream wraps camera.Camera.Stream.
func (c *WaitGroupCamera) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return c.Cam.Stream(ctx, errHandlers...)
}

// NextPointCloud wraps camera.Camera.NextPointCloud.
func (c *WaitGroupCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return c.Cam.NextPointCloud(ctx)
}

// Properties wraps camera.Camera.Properties.
func (c *WaitGroupCamera) Properties(ctx context.Context) (Properties, error) {
	return c.Cam.Properties(ctx)
}

// Close calls WaitGroup.Wait before calling camera.Camera.Close.
func (c *WaitGroupCamera) Close(ctx context.Context) error {
	c.ActiveBackgroundWorkers.Wait()
	return c.Cam.Close(ctx)
}
