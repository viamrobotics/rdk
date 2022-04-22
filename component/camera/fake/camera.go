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
			color := config.Attributes.String("color")
			cam := &Camera{Name: config.Name, color: color}
			return camera.New(cam, nil, nil)
		}})
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	Name  string
	color string
}

// Next always returns the same image with dot in the top left corner. The dot is colored
// either red (default), blue, or yellow.
func (c *Camera) Next(ctx context.Context) (image.Image, func(), error) {
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	switch c.color {
	case "blue":
		setDot(img, rimage.Blue)
	case "yellow":
		setDot(img, rimage.Yellow)
	default:
		setDot(img, rimage.Red)
	}
	return img, func() {}, nil
}

func setDot(img *image.NRGBA, color color.Color) {
	img.Set(16, 16, color)
	img.Set(16, 17, color)
	img.Set(16, 18, color)
	img.Set(17, 16, color)
	img.Set(17, 17, color)
	img.Set(17, 18, color)
	img.Set(18, 16, color)
	img.Set(18, 17, color)
	img.Set(18, 18, color)
}

// NextPointCloud always returns a pointcloud with a single pixel.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	pc := pointcloud.New()
	return pc, pc.Set(pointcloud.NewVector(16, 16, 16), pointcloud.NewColoredData(color.NRGBA{255, 0, 0, 255}))
}

// Do echos back whatever was sent to it.
func (c *Camera) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
