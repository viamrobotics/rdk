// Package fake implements a fake camera.
package fake

import (
	"context"
	"image"
	"image/color"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/vision"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			cam := &Camera{Name: config.Name}
			return camera.New(cam, nil, nil)
		}})
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	Name string
}

// Next always returns the same image with a red dot in the center.
func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	img.Set(16, 16, rimage.Red)
	return img, func() {}, nil
}

// NextPointCloud always returns a pointcloud with a single pixel.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	pc := pointcloud.New()
	return pc, pc.Set(pointcloud.NewColoredPoint(16, 16, 16, color.NRGBA{255, 0, 0, 255}))
}

// NextObjects always returns the single pixel as the only object in the scene.
func (c *Camera) NextObjects(ctx context.Context, params *vision.Parameters3D) ([]*vision.Object, error) {
	pc := pointcloud.New()
	err := pc.Set(pointcloud.NewColoredPoint(16, 16, 16, color.NRGBA{255, 0, 0, 255}))
	if err != nil {
		return nil, err
	}
	objs := []*vision.Object{vision.NewObject(pc)}
	return objs, nil
}
