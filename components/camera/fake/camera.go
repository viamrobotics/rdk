// Package fake implements a fake camera.
package fake

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		resource.NewDefaultModel("fake"),
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			res := config.Attributes.String("resolution")
			resModel, err := fakeModel(res)
			cam := &Camera{Name: config.Name, Model: fakeModel}
			return camera.NewFromReader(ctx, cam, fakeModel, camera.ColorStream)
		}})
}

var fakeIntrinsicsHigh = &transform.PinholeCameraIntrinsics{
	Width:  1024,
	Height: 768,
	Fx:     821.32642889,
	Fy:     821.68607359,
	Ppx:    494.95941428,
	Ppy:    370.70529534,
}
var fakeDistortionHigh = &transform.BrownConrady{
	RadialK1:     0.11297234,
	RadialK2:     -0.21375332,
	RadialK3:     -0.01584774,
	TangentialP1: -0.00302002,
	TangentialP2: 0.19969297,
}

var fakeIntrinsicsMed = &transform.PinholeCameraIntrinsics{
	Width:  512,
	Height: 384,
	Fx:     410.66321445,
	Fy:     410.84303680,
	Ppx:    247.47970714,
	Ppy:    185.35264767,
}

var fakeIntrinsicsLow = &transform.PinholeCameraIntrinsics{
	Width:  256,
	Height: 192,
	Fx:     205.3316072,
	Fy:     205.4215184,
	Ppx:    123.73985357,
	Ppy:    92.676323835,
}

var fakeModelHigh = &transform.PinholeCameraModel{
	PinholeCameraIntrinsics: fakeIntrinsicsHigh,
	Distortion:              fakeDistortionHigh,
}

var fakeModelMed = &transform.PinholeCameraModel{
	PinholeCameraIntrinsics: fakeIntrinsicsMed,
}

var fakeModelLow = &transform.PinholeCameraModel{
	PinholeCameraIntrinsics: fakeIntrinsicsLow,
}

func fakeModel(res string) (*transform.PinholeCameraModel, error) {
	switch res {
	case "high", "hi", "":
		return fakeModelHigh, nil
	case "medium", "med":
		return fakeModelMed, nil
	case "low", "lo":
		return fakeModelLow, nil
	default:
		return nil, errors.Errorf(`do not know resolution %q, only "high", "medium", or "low" are available`, res)
	}
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	generic.Echo
	Name       string
	Model      *transform.PinholeCameraModel
	Resolution string
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
