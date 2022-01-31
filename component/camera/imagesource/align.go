package imagesource

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(camera.Subtype, "depth_composed",
		registry.Component{Constructor: func(ctx context.Context, r robot.Robot,
			config config.Component, logger golog.Logger) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*rimage.AttrConfig)
			if !ok {
				return nil, errors.New("cannot retrieve converted attributes")
			}

			colorName := attrs.Color
			color, ok := r.CameraByName(colorName)
			if !ok {
				return nil, errors.Errorf("cannot find color camera (%s)", colorName)
			}

			depthName := attrs.Depth
			depth, ok := r.CameraByName(depthName)
			if !ok {
				return nil, errors.Errorf("cannot find depth camera (%s)", depthName)
			}

			dc, err := NewDepthComposed(color, depth, attrs, logger)
			if err != nil {
				return nil, err
			}
			return &camera.ImageSource{ImageSource: dc}, nil
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "depth_composed",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf rimage.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &rimage.AttrConfig{})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "depth_composed", "warp",
		func(val interface{}) (interface{}, error) {
			warp := &transform.AlignConfig{}
			err := mapstructure.Decode(val, warp)
			if err == nil {
				err = warp.CheckValid()
			}
			return warp, err
		})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "depth_composed", "intrinsic_extrinsic",
		func(val interface{}) (interface{}, error) {
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

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "depth_composed", "homography",
		func(val interface{}) (interface{}, error) {
			homography := &transform.RawPinholeCameraHomography{}
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: homography})
			if err != nil {
				return nil, err
			}
			err = decoder.Decode(val)
			if err == nil {
				err = homography.CheckValid()
			}
			return homography, err
		})
}

var alignCurrentlyWriting = false

func getCameraSystems(attrs *rimage.AttrConfig, logger golog.Logger) (rimage.Aligner, rimage.Projector, error) {
	var alignCamera rimage.Aligner
	var projectCamera rimage.Projector

	switch {
	case attrs.IntrinsicExtrinsic != nil:
		cam, err := transform.NewDepthColorIntrinsicsExtrinsics(*attrs)
		if err != nil {
			return nil, nil, err
		}
		alignCamera = cam
		projectCamera = cam
	case attrs.Homography != nil:
		conf, ok := attrs.Homography.(*transform.RawPinholeCameraHomography)
		if !ok {
			return nil, nil, rdkutils.NewUnexpectedTypeError(conf, attrs.Homography)
		}
		cam, err := transform.NewPinholeCameraHomography(conf)
		if err != nil {
			return nil, nil, err
		}
		alignCamera = cam
		projectCamera = cam
	case attrs.Warp != nil:
		conf, ok := attrs.Warp.(*transform.AlignConfig)
		if !ok {
			return nil, nil, rdkutils.NewUnexpectedTypeError(conf, attrs.Warp)
		}
		cam, err := transform.NewDepthColorWarpTransforms(conf, logger)
		if err != nil {
			return nil, nil, err
		}
		alignCamera = cam
		projectCamera = cam
	case attrs.Intrinsic != nil:
		alignCamera = nil
		var ok bool
		projectCamera, ok = attrs.Intrinsic.(*transform.PinholeCameraIntrinsics)
		if !ok {
			return nil, nil, rdkutils.NewUnexpectedTypeError(projectCamera, attrs.Intrinsic)
		}
	default: // default is no camera systems returned
		return nil, nil, nil
	}

	return alignCamera, projectCamera, nil
}

// depthComposed TODO.
type depthComposed struct {
	color, depth     gostream.ImageSource
	alignmentCamera  rimage.Aligner
	projectionCamera rimage.Projector
	aligned          bool
	debug            bool
	logger           golog.Logger
}

// NewDepthComposed TODO.
func NewDepthComposed(color, depth gostream.ImageSource, attrs *rimage.AttrConfig, logger golog.Logger) (gostream.ImageSource, error) {
	alignCamera, projectCamera, err := getCameraSystems(attrs, logger)
	if err != nil {
		return nil, err
	}
	if alignCamera == nil || projectCamera == nil {
		return nil, errors.New("missing camera system config")
	}

	return &depthComposed{color, depth, alignCamera, projectCamera, attrs.Aligned, attrs.Debug, logger}, nil
}

// Next TODO.
func (dc *depthComposed) Next(ctx context.Context) (image.Image, func(), error) {
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

	ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, dc.aligned, nil)

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
	if err != nil {
		return nil, nil, err
	}
	aligned.SetProjector(dc.projectionCamera)

	return aligned, func() {}, err
}
