package videosource

import (
	"context"
	"encoding/json"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
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
			var conf alignAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*alignAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
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
			b, err := json.Marshal(val)
			if err != nil {
				return nil, err
			}
			matrices, err := transform.NewDepthColorIntrinsicsExtrinsicsFromBytes(b)
			if err != nil {
				return nil, err
			}
			err = matrices.CheckValid()
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

func getAligner(attrs *alignAttrs, logger golog.Logger) (transform.Aligner, error) {
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
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Stream               string                             `json:"stream"`
	Debug                bool                               `json:"debug,omitempty"`
	Color                string                             `json:"color_camera_name"`
	Depth                string                             `json:"depth_camera_name"`
	Height               int                                `json:"height_px"`
	Width                int                                `json:"width_px"`
	IntrinsicExtrinsic   interface{}                        `json:"intrinsic_extrinsic,omitempty"`
	Homography           interface{}                        `json:"homography,omitempty"`
	Warp                 interface{}                        `json:"warp,omitempty"`
}

func (cfg *alignAttrs) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Color == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "color_camera_name")
	}
	deps = append(deps, cfg.Color)
	if cfg.Depth == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "depth_camera_name")
	}
	deps = append(deps, cfg.Depth)
	return deps, nil
}

// alignColorDepth takes a color and depth image source and aligns them together.
type alignColorDepth struct {
	color, depth gostream.VideoStream
	aligner      transform.Aligner
	projector    transform.Projector
	stream       camera.StreamType
	height       int // height of the aligned image
	width        int // width of the aligned image
	debug        bool
	logger       golog.Logger
}

// newAlignColorDepth creates a gostream.VideoSource that aligned color and depth channels.
func newAlignColorDepth(ctx context.Context, color, depth camera.Camera, attrs *alignAttrs, logger golog.Logger,
) (camera.Camera, error) {
	alignCamera, err := getAligner(attrs, logger)
	if err != nil {
		return nil, err
	}
	if attrs.Height <= 0 || attrs.Width <= 0 {
		return nil, errors.Errorf(
			"alignColorDepth needs Width and Height fields set. Got illegal dimensions (%d, %d)",
			attrs.Width,
			attrs.Height,
		)
	}
	// get the projector for the alignment camera
	stream := camera.StreamType(attrs.Stream)
	var props camera.Properties
	var intrinsicParams *transform.PinholeCameraIntrinsics
	switch {
	case attrs.CameraParameters != nil:
		intrinsicParams = attrs.CameraParameters
	case stream == camera.ColorStream, stream == camera.UnspecifiedStream:
		props, err = color.Properties(ctx)
		if err != nil {
			return nil, camera.NewPropertiesError("color camera")
		}
		intrinsicParams = props.IntrinsicParams
	case stream == camera.DepthStream:
		props, err = depth.Properties(ctx)
		if err != nil {
			return nil, camera.NewPropertiesError("depth camera")
		}
		intrinsicParams = props.IntrinsicParams
	default:
		return nil, camera.NewUnsupportedStreamError(stream)
	}

	videoSrc := &alignColorDepth{
		color:     gostream.NewEmbeddedVideoStream(color),
		depth:     gostream.NewEmbeddedVideoStream(depth),
		aligner:   alignCamera,
		projector: intrinsicParams,
		stream:    stream,
		height:    attrs.Height,
		width:     attrs.Width,
		debug:     attrs.Debug,
		logger:    logger,
	}
	return camera.NewFromReader(ctx, videoSrc, &transform.PinholeCameraModel{intrinsicParams, nil}, stream)
}

// Read aligns the next images from the color and the depth sources to the frame of the color camera.
// stream parameter will determine which channel gets streamed.
func (acd *alignColorDepth) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "videosource::alignColorDepth::Read")
	defer span.End()
	switch acd.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		// things are being aligned to the color image, so just return the color image.
		return acd.color.Next(ctx)
	case camera.DepthStream:
		// don't need to request the color image, just its dimensions
		colDimImage := rimage.NewImage(acd.width, acd.height)
		depth, depthCloser, err := acd.depth.Next(ctx)
		if err != nil {
			return nil, nil, err
		}
		dm, err := rimage.ConvertImageToDepthMap(ctx, depth)
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
	ctx, span := trace.StartSpan(ctx, "videosource::alignColorDepth::NextPointCloud")
	defer span.End()
	if acd.projector == nil {
		return nil, transform.NewNoIntrinsicsError("")
	}
	col, dm := camera.SimultaneousColorDepthNext(ctx, acd.color, acd.depth)
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

func (acd *alignColorDepth) Close(ctx context.Context) error {
	return multierr.Combine(acd.color.Close(ctx), acd.depth.Close(ctx))
}
