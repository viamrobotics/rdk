package imagesource

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/artifact"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"go.opencensus.io/trace"
)

func init() {
	registry.RegisterCamera("depthComposed", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
		attrs := config.Attributes

		colorName := attrs.String("color")
		color := r.CameraByName(colorName)
		if color == nil {
			return nil, errors.Errorf("cannot find color camera (%s)", colorName)
		}

		depthName := attrs.String("depth")
		depth := r.CameraByName(depthName)
		if depth == nil {
			return nil, errors.Errorf("cannot find depth camera (%s)", depthName)
		}

		dc, err := NewDepthComposed(color, depth, config.Attributes, logger)
		if err != nil {
			return nil, err
		}
		return &camera.ImageSource{dc}, nil
	})

	config.RegisterAttributeConverter(config.ComponentTypeCamera, "depthComposed", "config", func(val interface{}) (interface{}, error) {
		config := &transform.AlignConfig{}
		err := mapstructure.Decode(val, config)
		if err == nil {
			err = config.CheckValid()
		}
		return config, err
	})

	config.RegisterAttributeConverter(config.ComponentTypeCamera, "depthComposed", "matrices", func(val interface{}) (interface{}, error) {
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

// DepthComposed TODO
type DepthComposed struct {
	color, depth     gostream.ImageSource
	alignmentCamera  rimage.CameraSystem
	projectionCamera rimage.CameraSystem
	aligned          bool
	debug            bool
	logger           golog.Logger
}

// NewDepthComposed TODO
func NewDepthComposed(color, depth gostream.ImageSource, attrs config.AttributeMap, logger golog.Logger) (*DepthComposed, error) {
	var alignCamera rimage.CameraSystem
	var projectCamera rimage.CameraSystem
	var err error

	if attrs.Has("config") && attrs.Has("matrices") {
		config := attrs["config"].(*transform.AlignConfig)
		alignCamera, err = transform.NewDepthColorWarpTransforms(config, logger)
		if err != nil {
			return nil, err
		}
		projectCamera, err = transform.NewDepthColorIntrinsicsExtrinsics(attrs)
		if err != nil {
			return nil, err
		}
	} else if attrs.Has("config") {
		config := attrs["config"].(*transform.AlignConfig)
		alignCamera, err = transform.NewDepthColorWarpTransforms(config, logger)
		if err != nil {
			return nil, err
		}
		projectCamera = alignCamera
	} else if attrs.Has("matrices") {
		alignCamera, err = transform.NewDepthColorIntrinsicsExtrinsics(attrs)
		if err != nil {
			return nil, err
		}
		projectCamera = alignCamera
	} else {
		return nil, errors.New("no camera system config")
	}
	return &DepthComposed{color, depth, alignCamera, projectCamera, attrs.Bool("aligned", false), attrs.Bool("debug", false), logger}, nil
}

// Close does nothing.
func (dc *DepthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
}

// Next TODO
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

	ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, dc.aligned, dc.alignmentCamera)

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
	aligned, err := dc.alignmentCamera.AlignImageWithDepth(ii)
	aligned.SetCameraSystem(dc.projectionCamera)

	return aligned, func() {}, err

}
