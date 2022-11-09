// Package fake implements a fake camera.
package fake

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			cam := &Camera{Name: config.Name, Model: fakeModel}
			return camera.NewFromReader(ctx, cam, fakeModel, camera.ColorStream)
		}})
}

var fakeIntrinsics = &transform.PinholeCameraIntrinsics{
	Width:  1024,
	Height: 768,
	Fx:     821.32642889,
	Fy:     821.68607359,
	Ppx:    494.95941428,
	Ppy:    370.70529534,
}

var fakeDistortion = &transform.BrownConrady{
	RadialK1:     0.11297234,
	RadialK2:     -0.21375332,
	RadialK3:     -0.01584774,
	TangentialP1: -0.00302002,
	TangentialP2: 0.19969297,
}

var fakeModel = &transform.PinholeCameraModel{
	PinholeCameraIntrinsics: fakeIntrinsics,
	Distortion:              fakeDistortion,
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	generic.Echo
	Name  string
	Model *transform.PinholeCameraModel
}

// Read always returns the same image of a chess board.
func (c *Camera) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	return img, func() {}, err
}

// NextPointCloud always returns a pointcloud of the chess board.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	if err != nil {
		return nil, err
	}
	dm, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	if err != nil {
		return nil, err
	}
	return c.Model.RGBDToPointCloud(img, dm)
}
