package imagesource

import (
	"context"
	"fmt"
	"image"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rimage/transform"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"go.opencensus.io/trace"
)

func init() {
	api.RegisterCamera("depthComposed", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		attrs := config.Attributes

		colorName := attrs.String("color")
		color := r.CameraByName(colorName)
		if color == nil {
			return nil, fmt.Errorf("cannot find color camera (%s)", colorName)
		}

		depthName := attrs.String("depth")
		depth := r.CameraByName(depthName)
		if depth == nil {
			return nil, fmt.Errorf("cannot find depth camera (%s)", depthName)
		}

		return NewDepthComposed(color, depth, config.Attributes, logger)
	})

	api.Register(api.ComponentTypeCamera, "depthComposed", "config", func(val interface{}) (interface{}, error) {
		config := &transform.AlignConfig{}
		err := mapstructure.Decode(val, config)
		if err == nil {
			err = config.CheckValid()
		}
		return config, err
	})

	api.Register(api.ComponentTypeCamera, "depthComposed", "matrices", func(val interface{}) (interface{}, error) {
		matrices := &transform.DepthColorIntrinsicsExtrinsics{}
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
	camera       rimage.CameraSystem
	aligned      bool
	debug        bool
	logger       golog.Logger
}

func NewDepthComposed(color, depth gostream.ImageSource, attrs api.AttributeMap, logger golog.Logger) (*DepthComposed, error) {
	var camera rimage.CameraSystem
	var err error

	if attrs.Has("config") {
		config := attrs["config"].(*transform.AlignConfig)
		camera, err = transform.NewDepthColorWarpTransforms(config, logger)
	} else if attrs.Has("matrices") {
		camera, err = transform.NewDepthColorIntrinsicsExtrinsics(attrs)
	} else {
		return nil, fmt.Errorf("no camera system config")
	}
	if err != nil {
		return nil, err
	}
	return &DepthComposed{color, depth, camera, attrs.Bool("aligned", false), attrs.Bool("debug", false), logger}, nil
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

	ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, dc.aligned, dc.camera)

	_, span := trace.StartSpan(ctx, "AlignImageWithDepth")
	defer span.End()
	if dc.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			utils.PanicCapturingGo(func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := artifact.MustNewPath(fmt.Sprintf("rimage/imagesource/align-test-%d.both.gz", time.Now().Unix()))
				err := ii.WriteTo(fn)
				if err != nil {
					dc.logger.Debugf("error writing debug file: %s", err)
				} else {
					dc.logger.Debugf("wrote debug file to %s", fn)
				}
			})
		}
	}
	aligned, err := dc.camera.AlignImageWithDepth(ii)

	return aligned, func() {}, err

}
