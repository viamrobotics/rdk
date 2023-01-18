// Package fake implements a fake camera.
package fake

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/artifact"
	"golang.org/x/image/draw"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("fake")

const (
	high   = "high"
	medium = "medium"
	low    = "low"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		model,
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := cfg.ConvertedAttributes.(*Attrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, cfg.ConvertedAttributes)
			}
			resModel, err := fakeModel(attrs.Resolution)
			if err != nil {
				return nil, err
			}
			cam := &Camera{Name: cfg.Name, Model: resModel, Resolution: attrs.Resolution}
			return camera.NewFromReader(ctx, cam, resModel, camera.ColorStream)
		}})
	config.RegisterComponentAttributeMapConverter(
		camera.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Attrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*Attrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&Attrs{},
	)
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
	Fx:     410.663214445,
	Fy:     410.843036795,
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
	case high, "":
		return fakeModelHigh, nil
	case medium:
		return fakeModelMed, nil
	case low:
		return fakeModelLow, nil
	default:
		return nil, errors.Errorf(`do not know resolution %q, only "high", "medium", or "low" are available`, res)
	}
}

// Attrs are the attributes of the fake camera config.
type Attrs struct {
	Resolution string `json:"resolution"`
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
	img, err := c.getColorImage()
	return img, func() {}, err
}

// NextPointCloud always returns a pointcloud of the chess board.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	img, err := c.getColorImage()
	if err != nil {
		return nil, err
	}
	dm, err := c.getDepthImage(ctx)
	if err != nil {
		return nil, err
	}
	return c.Model.RGBDToPointCloud(img, dm)
}

// getColorImage always returns the same color image of a chess board.
func (c *Camera) getColorImage() (*rimage.Image, error) {
	switch c.Resolution {
	case high, "":
		img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
		return img, err
	case medium:
		img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2_med.png"))
		return img, err
	case low:
		img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2_low.png"))
		return img, err
	default:
		return nil, errors.Errorf(`do not know resolution %q, only "high", "medium", or "low" are available`, c.Resolution)
	}
}

// getDepthImage always returns the same depth image of a chess board.
func (c *Camera) getDepthImage(ctx context.Context) (*rimage.DepthMap, error) {
	img, err := rimage.NewDepthMapFromFile(ctx, artifact.MustPath("rimage/board2_gray.png"))
	if err != nil {
		return nil, err
	}
	switch c.Resolution {
	case high, "":
		return img, nil
	case medium:
		dm, err := c.resizeDepthImage(ctx, img, 640, 360)
		return dm, err
	case low:
		dm, err := c.resizeDepthImage(ctx, img, 320, 180)
		return dm, err
	default:
		return nil, errors.Errorf(`do not know resolution %q, only "high", "medium", or "low" are available`, c.Resolution)
	}
}

func (c *Camera) resizeDepthImage(ctx context.Context, dm *rimage.DepthMap, width, height int,
) (*rimage.DepthMap, error) {
	dm2, err := rimage.ConvertImageToGray16(dm)
	if err != nil {
		return nil, err
	}
	dst := image.NewGray16(image.Rect(0, 0, width, height))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), dm2, dm.Bounds(), draw.Over, nil)
	dmFinal, err := rimage.ConvertImageToDepthMap(ctx, dst)
	if err != nil {
		return nil, err
	}
	return dmFinal, nil
}
