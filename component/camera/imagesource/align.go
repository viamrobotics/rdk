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
	registry.RegisterComponent(camera.Subtype, "align_color_depth",
		registry.Component{Constructor: func(ctx context.Context, r robot.Robot,
			config config.Component, logger golog.Logger) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, errors.Errorf("expected config.ConvertedAttributes to be *camera.AttrConfig but got %T", config.ConvertedAttributes)
			}

			return newAlignColorDepth(attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "align_color_depth",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.AttrConfig{})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "align_color_depth", "warp",
		func(val interface{}) (interface{}, error) {
			warp := &transform.AlignConfig{}
			err := mapstructure.Decode(val, warp)
			if err == nil {
				err = warp.CheckValid()
			}
			return warp, err
		})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "align_color_depth", "intrinsic_extrinsic",
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

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "align_color_depth", "homography",
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

func getAligner(attrs *camera.AttrConfig, logger golog.Logger) (rimage.Aligner, error) {
	switch {
	case attrs.IntrinsicExtrinsic != nil:
		cam, ok := attrs.IntrinsicExtrinsic.(*transform.DepthColorIntrinsicsExtrinsics)
		if !ok {
			return nil, rdkutils.NewUnexpectedTypeError(cam, attrs.IntrinsicExtrinsic)
		}
		return cam, nil
	case attrs.Homography != nil:
		conf, ok := attrs.Homography.(*transform.RawPinholeCameraHomography)
		if !ok {
			return nil, nil, rdkutils.NewUnexpectedTypeError(conf, attrs.Homography)
		}
		cam, err := transform.NewPinholeCameraHomography(conf)
		if err != nil {
			return nil, err
		}
		return cam, nil
	case attrs.Warp != nil:
		conf, ok := attrs.Warp.(*transform.AlignConfig)
		if !ok {
			return nil, rdkutils.NewUnexpectedTypeError(conf, attrs.Warp)
		}
		cam, err := transform.NewDepthColorWarpTransforms(conf, logger)
		if err != nil {
			return nil, err
		}
		return cam, nil
	default:
		return nil, errors.New("no valid alignment attribute field provided")
	}
}

// alignColorDepth takes a color and depth image source and aligns them together.
type alignColorDepth struct {
	color, depth    gostream.ImageSource
	alignmentCamera rimage.Aligner
	debug           bool
	logger          golog.Logger
}

// newAlignColorDepth creates a gostream.ImageSource that aligned color and depth channels.
func newAlignColorDepth(attrs *camera.AttrConfig, logger golog.Logger) (camera.Camera, error) {
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

	alignCamera, err := getAligner(attrs, logger)
	if err != nil {
		return nil, err
	}

	imgSrc := &alignColorDepth{color, depth, alignCamera, attrs.Debug, logger}
	return camera.New(imgSrc, attrs, nil)
}

// Next aligns the next images from the color and the depth sources
func (dc *alignColorDepth) Next(ctx context.Context) (image.Image, func(), error) {
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

	ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, dc.aligned)

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
	return aligned, func() {}, err
}
