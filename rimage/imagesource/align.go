package imagesource

import (
	"context"
	"fmt"
	"image"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rimage/calib"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"go.opencensus.io/trace"
)

func init() {
	api.RegisterCamera("depthComposed", func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (gostream.ImageSource, error) {
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
		config := &calib.AlignConfig{}
		err := mapstructure.Decode(val, config)
		if err == nil {
			err = config.CheckValid()
		}
		return config, err
	})

	api.Register(api.ComponentTypeCamera, "depthComposed", "matrices", func(val interface{}) (interface{}, error) {
		matrices := &calib.DepthColorIntrinsicsExtrinsics{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: matrices})
		if err != nil {
			return nil, err
		}
		err = decoder.Decode(val)
		if err == nil {
			err = matrices.CheckValid()
		}
		return matrices, err
	})
}

var alignCurrentlyWriting = false

type DepthComposed struct {
	color, depth gostream.ImageSource
	aligner      rimage.DepthColorAligner
	debug        bool
	logger       golog.Logger
}

func NewDepthComposed(color, depth gostream.ImageSource, attrs api.AttributeMap, logger golog.Logger) (*DepthComposed, error) {
	var dcaligner rimage.DepthColorAligner
	var err error

	if attrs.Has("config") {
		config := attrs["config"].(*calib.AlignConfig)
		dcaligner, err = calib.NewDepthColorWarpTransforms(config, logger)
	} else if attrs.Has("matrices") {
		dcaligner, err = calib.NewDepthColorIntrinsicsExtrinsics(attrs)
	} else {
		return nil, fmt.Errorf("no alignment config")
	}
	if err != nil {
		return nil, err
	}
	return &DepthComposed{color, depth, dcaligner, attrs.GetBool("debug", false), logger}, nil
}

func (dc *DepthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
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

	dm, err := rimage.ConvertImageToDepthMap(d)
	if err != nil {
		return nil, nil, err
	}

	ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, false, nil)

	_, span := trace.StartSpan(ctx, "AlignImageWithDepth")
	defer span.End()
	if dc.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			go func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := artifact.MustNewPath(fmt.Sprintf("rimage/imagesource/align-test-%d.both.gz", time.Now().Unix()))
				err := ii.WriteTo(fn)
				if err != nil {
					dc.logger.Debugf("error writing debug file: %s", err)
				} else {
					dc.logger.Debugf("wrote debug file to %s", fn)
				}
			}()
		}
	}
	aligned, err := dc.aligner.AlignImageWithDepth(ii)

	return aligned, func() {}, err

}
