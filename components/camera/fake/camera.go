// Package fake implements a fake camera which always returns the same image with a user specified resolution.
package fake

import (
	"context"
	"image"
	"image/color"
	"math"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

var model = resource.DefaultModelFamily.WithModel("fake")

const (
	initialWidth  = 1280
	initialHeight = 720
)

func init() {
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				cfg resource.Config,
				logger golog.Logger,
			) (camera.Camera, error) {
				return NewCamera(ctx, cfg, logger)
			},
		})
}

// NewCamera returns a new fake camera.
func NewCamera(
	ctx context.Context,
	conf resource.Config,
	logger golog.Logger,
) (camera.Camera, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	_, paramErr := newConf.Validate("")
	if paramErr != nil {
		return nil, paramErr
	}
	resModel, width, height := fakeModel(newConf.Width, newConf.Height)
	cam := &Camera{
		Named:  conf.ResourceName().AsNamed(),
		Model:  resModel,
		Width:  width,
		Height: height,
	}
	src, err := camera.NewVideoSourceFromReader(ctx, cam, resModel, camera.ColorStream)
	if err != nil {
		return nil, err
	}
	return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
}

// Config are the attributes of the fake camera config.
type Config struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// Validate checks that the config attributes are valid for a fake camera.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Height%2 != 0 {
		return nil, errors.Errorf("odd-number resolutions cannot be rendered, cannot use a height of %d", conf.Height)
	}
	if conf.Width%2 != 0 {
		return nil, errors.Errorf("odd-number resolutions cannot be rendered, cannot use a width of %d", conf.Width)
	}
	return nil, nil
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
	resource.Named
	resource.AlwaysRebuild
	Model           *transform.PinholeCameraModel
	Width           int
	Height          int
	cacheImage      *image.RGBA
	cachePointCloud pointcloud.PointCloud
}

// Read always returns the same image of a yellow to blue gradient.
func (c *Camera) Read(ctx context.Context) (image.Image, func(), error) {
	if c.cacheImage != nil {
		return c.cacheImage, func() {}, nil
	}
	width := float64(c.Width)
	height := float64(c.Height)
	img := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))

	totalDist := math.Sqrt(math.Pow(0-width, 2) + math.Pow(0-height, 2))

	var x, y float64
	for x = 0; x < width; x++ {
		for y = 0; y < height; y++ {
			dist := math.Sqrt(math.Pow(0-x, 2) + math.Pow(0-y, 2))
			dist /= totalDist
			thisColor := color.RGBA{uint8(255 - (255 * dist)), uint8(255 - (255 * dist)), uint8(0 + (255 * dist)), 255}
			img.Set(int(x), int(y), thisColor)
		}
	}
	c.cacheImage = img
	return rimage.ConvertImage(img), func() {}, nil
}

// NextPointCloud always returns a pointcloud of a yellow to blue gradient, with the depth determined by the intensity of blue.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c.cachePointCloud != nil {
		return c.cachePointCloud, nil
	}
	dm := pointcloud.New()
	width := float64(c.Width)
	height := float64(c.Height)

	totalDist := math.Sqrt(math.Pow(0-width, 2) + math.Pow(0-height, 2))

	var x, y float64
	for x = 0; x < width; x++ {
		for y = 0; y < height; y++ {
			dist := math.Sqrt(math.Pow(0-x, 2) + math.Pow(0-y, 2))
			dist /= totalDist
			thisColor := color.NRGBA{uint8(255 - (255 * dist)), uint8(255 - (255 * dist)), uint8(0 + (255 * dist)), 255}
			err := dm.Set(r3.Vector{X: x, Y: y, Z: 255 * dist}, pointcloud.NewColoredData(thisColor))
			if err != nil {
				return nil, err
			}
		}
	}
	c.cachePointCloud = dm
	return dm, nil
}

// Close does nothing.
func (c *Camera) Close(ctx context.Context) error {
	return nil
}
