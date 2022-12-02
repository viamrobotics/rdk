package videosource

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
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
	registry.RegisterComponent(camera.Subtype, "join_color_depth",
		registry.Component{Constructor: func(ctx context.Context, deps registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*joinAttrs)
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
			return newJoinColorDepth(ctx, color, depth, attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "join_color_depth",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf joinAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*joinAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		}, &joinAttrs{})
}

// joinAttrs is the attribute struct for aligning.
type joinAttrs struct {
	ImageType            string                             `json:"output_image_type"`
	Debug                bool                               `json:"debug,omitempty"`
	Color                string                             `json:"color_camera_name"`
	Depth                string                             `json:"depth_camera_name"`
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

func (cfg *joinAttrs) Validate(path string) ([]string, error) {
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

// joinColorDepth takes a color and depth image source and aligns them together.
type joinColorDepth struct {
	color, depth gostream.VideoStream
	projector    transform.Projector
	imageType    camera.StreamType
	debug        bool
	logger       golog.Logger
}

// newJoinColorDepth creates a gostream.VideoSource that aligned color and depth channels.
func newJoinColorDepth(ctx context.Context, color, depth camera.Camera, attrs *joinAttrs, logger golog.Logger,
) (camera.Camera, error) {

	videoSrc := &joinColorDepth{
		color:     gostream.NewEmbeddedVideoStream(color),
		depth:     gostream.NewEmbeddedVideoStream(depth),
		projector: intrinsicParams,
		imageType: stream,
		debug:     attrs.Debug,
		logger:    logger,
	}
	return camera.NewFromReader(ctx, videoSrc, &transform.PinholeCameraModel{intrinsicParams, nil}, stream)
}

// Read aligns the next images from the color and the depth sources to the frame of the color camera.
// stream parameter will determine which channel gets streamed.
func (jcd *joinColorDepth) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "videosource::joinColorDepth::Read")
	defer span.End()
	switch jcd.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		// things are being aligned to the color image, so just return the color image.
		return jcd.color.Next(ctx)
	case camera.DepthStream:
		// don't need to request the color image, just its dimensions
		colDimImage := rimage.NewImage(jcd.width, jcd.height)
		depth, depthCloser, err := jcd.depth.Next(ctx)
		if err != nil {
			return nil, nil, err
		}
		dm, err := rimage.ConvertImageToDepthMap(ctx, depth)
		if err != nil {
			return nil, nil, err
		}
		if jcd.aligner == nil {
			return dm, depthCloser, nil
		}
		_, alignedDepth, err := jcd.aligner.AlignColorAndDepthImage(colDimImage, dm)
		return alignedDepth, depthCloser, err
	default:
		return nil, nil, camera.NewUnsupportedStreamError(jcd.stream)
	}
}

func (jcd *joinColorDepth) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "videosource::joinColorDepth::NextPointCloud")
	defer span.End()
	if jcd.projector == nil {
		return nil, transform.NewNoIntrinsicsError("")
	}
	col, dm := camera.SimultaneousColorDepthNext(ctx, jcd.color, jcd.depth)
	if col == nil || dm == nil {
		return nil, errors.New("requested color or depth image from camera is nil")
	}
	if jcd.aligner == nil {
		return jcd.projector.RGBDToPointCloud(rimage.ConvertImage(col), dm)
	}
	alignedColor, alignedDepth, err := jcd.aligner.AlignColorAndDepthImage(rimage.ConvertImage(col), dm)
	if err != nil {
		return nil, err
	}
	return jcd.projector.RGBDToPointCloud(alignedColor, alignedDepth)
}

func (jcd *joinColorDepth) Close(ctx context.Context) error {
	return multierr.Combine(jcd.color.Close(ctx), jcd.depth.Close(ctx))
}
