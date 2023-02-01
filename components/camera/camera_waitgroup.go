package camera

import (
	"context"
	"sync"

	"github.com/edaniels/gostream"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage/transform"
)

// CameraWaitGroup is a wrapper for camera.Camera with a sync.WaitGroup.
type CameraWaitGroup struct {
	Cam                     Camera
	ActiveBackgroundWorkers sync.WaitGroup
}

// DoCommand wraps camera.Camera.DoCommand.
func (c *CameraWaitGroup) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return c.Cam.DoCommand(ctx, cmd)
}

// Projector wraps camera.Camera.Projector.
func (c *CameraWaitGroup) Projector(ctx context.Context) (transform.Projector, error) {
	return c.Cam.Projector(ctx)
}

// Stream wraps camera.Camera.Stream.
func (c *CameraWaitGroup) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return c.Cam.Stream(ctx, errHandlers...)
}

// NextPointCloud wraps camera.Camera.NextPointCloud.
func (c *CameraWaitGroup) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return c.Cam.NextPointCloud(ctx)
}

// Properties wraps camera.Camera.Properties.
func (c *CameraWaitGroup) Properties(ctx context.Context) (Properties, error) {
	return c.Cam.Properties(ctx)
}

// Close calls WaitGroup.Wait before calling camera.Camera.Close.
func (c *CameraWaitGroup) Close(ctx context.Context) error {
	c.ActiveBackgroundWorkers.Wait()
	return c.Cam.Close(ctx)
}
