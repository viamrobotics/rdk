package imagesource

import (
	"context"
	"fmt"
	"image"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/calibration"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
)

func init() {
	api.RegisterCamera("depthComposed", func(r api.Robot, config api.Component, logger golog.Logger) (gostream.ImageSource, error) {
		attrs := config.Attributes

		colorName := attrs.GetString("color")
		color := r.CameraByName(colorName)
		if color == nil {
			return nil, fmt.Errorf("cannot find color camera (%s)", colorName)
		}

		depthName := attrs.GetString("depth")
		depth := r.CameraByName(depthName)
		if depth == nil {
			return nil, fmt.Errorf("cannot find depth camera (%s)", depthName)
		}

		return NewDepthComposed(color, depth, config.Attributes, logger)
	})

	api.Register(api.ComponentTypeCamera, "depthComposed", "config", func(val interface{}) (interface{}, error) {
		config := &rimage.AlignConfig{}
		err := mapstructure.Decode(val, config)
		if err == nil {
			err = config.CheckValid()
		}
		return config, err
	})
}

type DepthComposed struct {
	color, depth                      gostream.ImageSource
	*calibration.DepthColorTransforms // using anonymous fields
	logger                            golog.Logger
}

func NewDepthComposed(color, depth gostream.ImageSource, attrs api.AttributeMap, logger golog.Logger) (*DepthComposed, error) {
	dct, err := calibration.NewDepthColorTransforms(attrs, logger)
	if err != nil {
		return nil, err
	}
	return &DepthComposed{color, depth, dct, logger}, nil
}

func (dc *DepthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
}

func convertImageToDepthMap(img image.Image) (*rimage.DepthMap, error) {
	switch ii := img.(type) {
	case *rimage.ImageWithDepth:
		return ii.Depth, nil
	case *image.Gray16:
		return imageToDepthMap(ii), nil
	default:
		return nil, fmt.Errorf("don't know how to make DepthMap from %T", img)
	}
}

func (dc *DepthComposed) Next(ctx context.Context) (image.Image, func(), error) {
	c, cCloser, err := dc.color.Next(ctx)
	if err != nil {
		return nil, nil, err
	}

	defer cCloser()

	d, dCloser, err := dc.depth.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer dCloser()

	dm, err := convertImageToDepthMap(d)
	if err != nil {
		return nil, nil, err
	}

	aligned, err := dc.AlignColorAndDepth(ctx, &rimage.ImageWithDepth{rimage.ConvertImage(c), dm}, dc.logger)

	return aligned, func() {}, err

}
