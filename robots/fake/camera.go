package fake

import (
	"context"
	"image"
	"image/color"

	"github.com/edaniels/golog"

	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/pointcloud"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterCamera("fake", registry.Camera{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
		return &Camera{Name: config.Name, FrameConfig: config.Frame}, nil
	}})
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	Name        string
	FrameConfig *config.Frame
}

// Next always returns the same image with a red dot in the center.
func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	img.Set(16, 16, rimage.Red)
	return img, func() {}, nil
}

// NextPointCloud always returns a pointcloud with a single pixel
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	pc := pointcloud.New()
	return pc, pc.Set(pointcloud.NewColoredPoint(16, 16, 16, color.NRGBA{255, 0, 0, 255}))
}

// Close does nothing.
func (c *Camera) Close() error {
	return nil
}

// FrameSystemLink has the info needed to add the camera to a frame system
func (c *Camera) FrameSystemLink() (*config.Frame, referenceframe.Frame) {
	return c.FrameConfig, nil
}
