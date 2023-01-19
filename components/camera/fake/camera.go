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
	initialWidth  = 1280
	initialHeight = 720
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
			if attrs.Height%2 != 0 {
				return nil, errors.Errorf("odd-number resolutions cannot be rendered, cannot use a height of %d", attrs.Height)
			}
			if attrs.Width%2 != 0 {
				return nil, errors.Errorf("odd-number resolutions cannot be rendered, cannot use a width of %d", attrs.Width)
			}
			resModel, width, height := fakeModel(attrs.Width, attrs.Height)
			cam := &Camera{
				Name:   cfg.Name,
				Model:  resModel,
				Width:  width,
				Height: height,
			}
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

// Attrs are the attributes of the fake camera config.
type Attrs struct {
	Width  int `json:"width"`
	Height int `json:"height"`
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

func fakeModel(width, height int) (*transform.PinholeCameraModel, int, int) {
	fakeModelReshaped := &transform.PinholeCameraModel{
		PinholeCameraIntrinsics: fakeIntrinsics,
		Distortion:              fakeDistortion,
	}
	switch {
	case width > 0 && height > 0:
		widthRatio := float64(width) / float64(initialWidth)
		heightRatio := float64(height) / float64(initialHeight)
		intrinsics := &transform.PinholeCameraIntrinsics{
			Width:  int(float64(fakeIntrinsics.Width) * widthRatio),
			Height: int(float64(fakeIntrinsics.Height) * heightRatio),
			Fx:     fakeIntrinsics.Fx * widthRatio,
			Fy:     fakeIntrinsics.Fy * heightRatio,
			Ppx:    fakeIntrinsics.Ppx * widthRatio,
			Ppy:    fakeIntrinsics.Ppy * heightRatio,
		}
		fakeModelReshaped.PinholeCameraIntrinsics = intrinsics
		return fakeModelReshaped, width, height
	case width > 0 && height <= 0:
		ratio := float64(width) / float64(initialWidth)
		intrinsics := &transform.PinholeCameraIntrinsics{
			Width:  int(float64(fakeIntrinsics.Width) * ratio),
			Height: int(float64(fakeIntrinsics.Height) * ratio),
			Fx:     fakeIntrinsics.Fx * ratio,
			Fy:     fakeIntrinsics.Fy * ratio,
			Ppx:    fakeIntrinsics.Ppx * ratio,
			Ppy:    fakeIntrinsics.Ppy * ratio,
		}
		fakeModelReshaped.PinholeCameraIntrinsics = intrinsics
		newHeight := int(float64(initialHeight) * ratio)
		if newHeight%2 != 0 {
			newHeight++
		}
		return fakeModelReshaped, width, newHeight
	case width <= 0 && height > 0:
		ratio := float64(height) / float64(initialHeight)
		intrinsics := &transform.PinholeCameraIntrinsics{
			Width:  int(float64(fakeIntrinsics.Width) * ratio),
			Height: int(float64(fakeIntrinsics.Height) * ratio),
			Fx:     fakeIntrinsics.Fx * ratio,
			Fy:     fakeIntrinsics.Fy * ratio,
			Ppx:    fakeIntrinsics.Ppx * ratio,
			Ppy:    fakeIntrinsics.Ppy * ratio,
		}
		fakeModelReshaped.PinholeCameraIntrinsics = intrinsics
		newWidth := int(float64(initialWidth) * ratio)
		if newWidth%2 != 0 {
			newWidth++
		}
		return fakeModelReshaped, newWidth, height
	default:
		return fakeModelReshaped, initialWidth, initialHeight
	}
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	generic.Echo
	Name   string
	Model  *transform.PinholeCameraModel
	Width  int
	Height int
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
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	if err != nil {
		return nil, err
	}
	if c.Height == initialHeight && c.Width == initialWidth {
		return img, nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return rimage.ConvertImage(dst), nil
}

// getDepthImage always returns the same depth image of a chess board.
func (c *Camera) getDepthImage(ctx context.Context) (*rimage.DepthMap, error) {
	dm, err := rimage.NewDepthMapFromFile(ctx, artifact.MustPath("rimage/board2_gray.png"))
	if err != nil {
		return nil, err
	}
	if c.Height == initialHeight && c.Width == initialWidth {
		return dm, nil
	}
	dm2, err := rimage.ConvertImageToGray16(dm)
	if err != nil {
		return nil, err
	}
	dst := image.NewGray16(image.Rect(0, 0, c.Width, c.Height))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), dm2, dm2.Bounds(), draw.Over, nil)
	return rimage.ConvertImageToDepthMap(ctx, dst)
}
