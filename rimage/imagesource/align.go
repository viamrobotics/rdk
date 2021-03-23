package imagesource

import (
	"context"
	"fmt"
	"image"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"go.opencensus.io/trace"
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
	color, depth                   gostream.ImageSource
	colorTransform, depthTransform rimage.TransformationMatrix

	config *rimage.AlignConfig
	debug  bool
	logger golog.Logger
}

func NewDepthComposed(color, depth gostream.ImageSource, attrs api.AttributeMap, logger golog.Logger) (*DepthComposed, error) {
	var config *rimage.AlignConfig
	var err error

	if attrs.Has("config") {
		config = attrs["config"].(*rimage.AlignConfig)
	} else if attrs["make"] == "intel515" {
		config = &intelConfig
	} else {
		return nil, fmt.Errorf("no aligntmnt config")
	}

	dst := rimage.ArrayToPoints([]image.Point{{0, 0}, {config.OutputSize.X, config.OutputSize.Y}})

	if config.WarpFromCommon {
		config, err = config.ComputeWarpFromCommon(logger)
		if err != nil {
			return nil, err
		}
	}

	colorPoints := rimage.ArrayToPoints(config.ColorWarpPoints)
	depthPoints := rimage.ArrayToPoints(config.DepthWarpPoints)

	colorTransform := rimage.GetPerspectiveTransform(colorPoints, dst)
	depthTransform := rimage.GetPerspectiveTransform(depthPoints, dst)

	return &DepthComposed{color, depth, colorTransform, depthTransform, config, attrs.GetBool("debug", false), logger}, nil
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

	aligned, err := dc.alignColorAndDepth(ctx, &rimage.ImageWithDepth{rimage.ConvertImage(c), dm}, dc.logger)

	return aligned, func() {}, err

}

var (
	alignCurrentlyWriting = false
	intelConfig           = rimage.AlignConfig{
		ColorInputSize:  image.Point{1280, 720},
		ColorWarpPoints: []image.Point{{0, 0}, {1196, 720}},

		DepthInputSize:  image.Point{1024, 768},
		DepthWarpPoints: []image.Point{{67, 100}, {1019, 665}},

		OutputSize: image.Point{640, 360},
	}
)

func (dc *DepthComposed) alignColorAndDepth(ctx context.Context, ii *rimage.ImageWithDepth, logger golog.Logger) (*rimage.ImageWithDepth, error) {
	_, span := trace.StartSpan(ctx, "alignColorAndDepth")
	defer span.End()

	if dc.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			go func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := fmt.Sprintf("data/align-test-%d.both.gz", time.Now().Unix())
				err := ii.WriteTo(fn)
				if err != nil {
					logger.Debugf("error writing debug file: %s", err)
				} else {
					logger.Debugf("wrote debug file to %s", fn)
				}
			}()
		}
	}

	if ii.Color.Width() != dc.config.ColorInputSize.X ||
		ii.Color.Height() != dc.config.ColorInputSize.Y ||
		ii.Depth.Width() != dc.config.DepthInputSize.X ||
		ii.Depth.Height() != dc.config.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dc.config)
	}

	ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average

	c2 := rimage.WarpImage(ii, dc.colorTransform, dc.config.OutputSize)
	dm2 := ii.Depth.Warp(dc.depthTransform, dc.config.OutputSize)

	return &rimage.ImageWithDepth{c2, &dm2}, nil
}
