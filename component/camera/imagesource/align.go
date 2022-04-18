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
			attrs, ok := config.ConvertedAttributes.(*alignAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			colorName := attrs.Color
			color, err := camera.FromRobot(r, colorName)
			if err != nil {
				return nil, fmt.Errorf("no color camera (%s): %w", colorName, err)
			}

			depthName := attrs.Depth
			depth, err := camera.FromRobot(r, depthName)
			if err != nil {
				return nil, fmt.Errorf("no depth camera (%s): %w", depthName, err)
			}
			return newAlignColorDepth(color, depth, attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "align_color_depth",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf alignAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*alignAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		}, &alignAttrs{})

	config.RegisterComponentAttributeConverter(camera.SubtypeName, "align_color_depth", "warp",
		func(val interface{}) (interface{}, error) {
			warp := &transform.AlignConfig{}
			err := mapstructure.Decode(val, warp)
			if err == nil {
				err = warp.CheckValid()
			}
			return warp, err
		})

	config.RegisterComponentAttributeConverter(camera.SubtypeName, "align_color_depth", "intrinsic_extrinsic",
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

	config.RegisterComponentAttributeConverter(camera.SubtypeName, "align_color_depth", "homography",
		func(val interface{}) (interface{}, error) {
			homography := &transform.RawDepthColorHomography{}
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

func getAligner(attrs *alignAttrs, logger golog.Logger) (rimage.Aligner, error) {
	switch {
	case attrs.IntrinsicExtrinsic != nil:
		cam, ok := attrs.IntrinsicExtrinsic.(*transform.DepthColorIntrinsicsExtrinsics)
		if !ok {
			return nil, rdkutils.NewUnexpectedTypeError(cam, attrs.IntrinsicExtrinsic)
		}
		return cam, nil
	case attrs.Homography != nil:
		conf, ok := attrs.Homography.(*transform.RawDepthColorHomography)
		if !ok {
			return nil, rdkutils.NewUnexpectedTypeError(conf, attrs.Homography)
		}
		cam, err := transform.NewDepthColorHomography(conf)
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

// alignAttrs is the attribute struct for aligning.
type alignAttrs struct {
	*camera.AttrConfig
	Color              string      `json:"color"`
	Depth              string      `json:"depth"`
	IntrinsicExtrinsic interface{} `json:"intrinsic_extrinsic"`
	Homography         interface{} `json:"homography"`
	Warp               interface{} `json:"warp"`
}

// alignColorDepth takes a color and depth image source and aligns them together.
type alignColorDepth struct {
	color, depth    gostream.ImageSource
	alignmentCamera rimage.Aligner
	debug           bool
	logger          golog.Logger
}

// newAlignColorDepth creates a gostream.ImageSource that aligned color and depth channels.
func newAlignColorDepth(color, depth camera.Camera, attrs *alignAttrs, logger golog.Logger) (camera.Camera, error) {
	alignCamera, err := getAligner(attrs, logger)
	if err != nil {
		return nil, err
	}

	imgSrc := &alignColorDepth{color, depth, alignCamera, attrs.Debug, logger}
	return camera.New(imgSrc, attrs.AttrConfig, color) // aligns the image to the color camera
}

// Next aligns the next images from the color and the depth sources.
func (acd *alignColorDepth) Next(ctx context.Context) (image.Image, func(), error) {
	c, cCloser, err := acd.color.Next(ctx)
	if err != nil {
		return nil, nil, err
	}

	defer cCloser()

	d, dCloser, err := acd.depth.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer dCloser()

	dm, err := rimage.ConvertImageToDepthMap(d)
	if err != nil {
		return nil, nil, err
	}
	_, span := trace.StartSpan(ctx, "AlignImageWithDepth")
	defer span.End()
	if acd.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			utils.PanicCapturingGo(func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := artifact.MustNewPath(fmt.Sprintf("rimage/imagesource/align-test-%d.both.gz", time.Now().Unix()))
				ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, false)
				err := ii.WriteTo(fn)
				if err != nil {
					acd.logger.Debugf("error writing debug file: %s", err)
				} else {
					acd.logger.Debugf("wrote debug file to %s", fn)
				}
			})
		}
	}

	aligned, err := acd.alignmentCamera.AlignColorAndDepthImage(rimage.ConvertImage(c), dm)
	if err != nil {
		return nil, nil, err
	}
	return aligned, func() {}, err
}
