package imagesource

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(camera.Subtype, "align_color_depth",
		registry.Component{Constructor: func(ctx context.Context, deps registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*alignAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			colorName := attrs.Color
			color, err := camera.FromDependencies(deps, colorName)
			if err != nil {
				return nil, fmt.Errorf("no color camera (%s): %w", colorName, err)
			}

			depthName := attrs.Depth
			depth, err := camera.FromDependencies(deps, depthName)
			if err != nil {
				return nil, fmt.Errorf("no depth camera (%s): %w", depthName, err)
			}
			return newAlignColorDepth(ctx, color, depth, attrs, logger)
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
		return nil, nil
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
	color, depth gostream.ImageSource
	aligner      rimage.Aligner
	projector    rimage.Projector
	stream       camera.StreamType
	height       int // height of the aligned image
	width        int // width of the aligned image
	debug        bool
	logger       golog.Logger
}

// newAlignColorDepth creates a gostream.ImageSource that aligned color and depth channels.
func newAlignColorDepth(ctx context.Context, color, depth camera.Camera, attrs *alignAttrs, logger golog.Logger,
) (camera.Camera, error) {
	alignCamera, err := getAligner(attrs, logger)
	if err != nil {
		return nil, err
	}
	if attrs.Height <= 0 || attrs.Width <= 0 {
		return nil, errors.Errorf("alignColorDepth needs Width and Height fields set. Got illegal dimensions (%d, %d)", attrs.Width, attrs.Height)
	}
	// get the projector for the alignment camera
	stream := camera.StreamType(attrs.Stream)
	var proj rimage.Projector
	switch stream {
	case camera.ColorStream, camera.UnspecifiedStream, camera.BothStream:
		proj, _ = camera.GetProjector(ctx, attrs.AttrConfig, color)
	case camera.DepthStream:
		proj, _ = camera.GetProjector(ctx, attrs.AttrConfig, depth)
	default:
		return nil, camera.NewUnsupportedStreamError(stream)
	}

	imgSrc := &alignColorDepth{
		color:     color,
		depth:     depth,
		aligner:   alignCamera,
		projector: proj,
		stream:    stream,
		height:    attrs.Height,
		width:     attrs.Width,
		debug:     attrs.Debug,
		logger:    logger,
	}
	return camera.New(imgSrc, proj)
}

// Next aligns the next images from the color and the depth sources to the frame of the color camera.
// stream parameter will determine which channel gets streamed.
func (acd *alignColorDepth) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::alignColorDepth::Next")
	defer span.End()
	switch acd.stream {
	case camera.ColorStream, camera.UnspecifiedStream, camera.BothStream:
		// things are being aligned to the color image, so just return the color image.
		return acd.color.Next(ctx)
	case camera.DepthStream:
		// don't need to request the color image, just its dimensions
		colDimImage := rimage.NewImage(acd.width, acd.height)
		depth, depthCloser, err := acd.depth.Next(ctx)
		if err != nil {
			return nil, nil, err
		}
		dm, err := rimage.ConvertImageToDepthMap(depth)
		if err != nil {
			return nil, nil, err
		}
		if acd.aligner == nil {
			return dm, depthCloser, nil
		}
		_, alignedDepth, err := acd.aligner.AlignColorAndDepthImage(colDimImage, dm)
		return alignedDepth, depthCloser, err
	default:
		return nil, nil, camera.NewUnsupportedStreamError(acd.stream)
	}
}

func (acd *alignColorDepth) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::alignColorDepth::NextPointCloud")
	defer span.End()
	if acd.projector == nil {
		return nil, transform.NewNoIntrinsicsError("")
	}
	col, dm := camera.SimultaneousColorDepthRequest(ctx, acd.color, acd.depth)
	if col == nil || dm == nil {
		return nil, errors.New("requested color or depth image from camera is nil")
	}
	if acd.aligner == nil {
		return acd.projector.RGBDToPointCloud(rimage.ConvertImage(col), dm)
	}
	alignedColor, alignedDepth, err := acd.aligner.AlignColorAndDepthImage(rimage.ConvertImage(col), dm)
	if err != nil {
		return nil, err
	}
	return acd.projector.RGBDToPointCloud(alignedColor, alignedDepth)
}
