package align

import (
	"context"
	"encoding/json"
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
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

var extrinsicsModel = resource.NewDefaultModel("align_color_depth_extrinsics")

func init() {
	registry.RegisterComponent(camera.Subtype, extrinsicsModel,
		registry.Component{Constructor: func(ctx context.Context, deps registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*extrinsicsAttrs)
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
			return newColorDepthExtrinsics(ctx, color, depth, attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.Subtype, extrinsicsModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf extrinsicsAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*extrinsicsAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		}, &extrinsicsAttrs{})

	config.RegisterComponentAttributeConverter(camera.Subtype, extrinsicsModel, "camera_system",
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
}

// extrinsicsAttrs is the attribute struct for aligning.
type extrinsicsAttrs struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	IntrinsicExtrinsic   interface{}                        `json:"camera_system"`
	ImageType            string                             `json:"output_image_type"`
	Color                string                             `json:"color_camera_name"`
	Depth                string                             `json:"depth_camera_name"`
	Debug                bool                               `json:"debug,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

func (cfg *extrinsicsAttrs) Validate(path string) ([]string, error) {
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

// colorDepthExtrinsics takes a color and depth image source and aligns them together.
type colorDepthExtrinsics struct {
	color, depth         gostream.VideoStream
	colorName, depthName string
	aligner              transform.Aligner
	projector            transform.Projector
	imageType            camera.ImageType
	height               int // height of the aligned image
	width                int // width of the aligned image
	debug                bool
	logger               golog.Logger
}

// newColorDepthExtrinsics creates a gostream.VideoSource that aligned color and depth channels.
func newColorDepthExtrinsics(ctx context.Context, color, depth camera.Camera, attrs *extrinsicsAttrs, logger golog.Logger,
) (camera.Camera, error) {
	alignment, ok := attrs.IntrinsicExtrinsic.(*transform.DepthColorIntrinsicsExtrinsics)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(alignment, attrs.IntrinsicExtrinsic)
	}
	if attrs.CameraParameters == nil {
		return nil, transform.ErrNoIntrinsics
	}
	if attrs.CameraParameters.Height <= 0 || attrs.CameraParameters.Width <= 0 {
		return nil, errors.Errorf(
			"colorDepthExtrinsics needs Width and Height fields set in intrinsic_parameters. Got illegal dimensions (%d, %d)",
			attrs.CameraParameters.Width,
			attrs.CameraParameters.Height,
		)
	}
	// get the projector for the alignment camera
	imgType := camera.ImageType(attrs.ImageType)
	videoSrc := &colorDepthExtrinsics{
		color:     gostream.NewEmbeddedVideoStream(color),
		colorName: attrs.Color,
		depth:     gostream.NewEmbeddedVideoStream(depth),
		depthName: attrs.Depth,
		aligner:   alignment,
		projector: attrs.CameraParameters,
		imageType: imgType,
		height:    attrs.CameraParameters.Height,
		width:     attrs.CameraParameters.Width,
		debug:     attrs.Debug,
		logger:    logger,
	}
	return camera.NewFromReader(
		ctx,
		videoSrc,
		&transform.PinholeCameraModel{attrs.CameraParameters, attrs.DistortionParameters},
		imgType,
	)
}

// Read aligns the next images from the color and the depth sources to the frame of the color camera.
// stream parameter will determine which channel gets streamed.
func (cde *colorDepthExtrinsics) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "align::colorDepthExtrinsics::Read")
	defer span.End()
	switch cde.imageType {
	case camera.ColorStream, camera.UnspecifiedStream:
		// things are being aligned to the color image, so just return the color image.
		return cde.color.Next(ctx)
	case camera.DepthStream:
		// don't need to request the color image, just its dimensions
		colDimImage := rimage.NewImage(cde.width, cde.height)
		depth, depthCloser, err := cde.depth.Next(ctx)
		if err != nil {
			return nil, nil, err
		}
		dm, err := rimage.ConvertImageToDepthMap(ctx, depth)
		if err != nil {
			return nil, nil, err
		}
		if cde.aligner == nil {
			return dm, depthCloser, nil
		}
		_, alignedDepth, err := cde.aligner.AlignColorAndDepthImage(colDimImage, dm)
		return alignedDepth, depthCloser, err
	default:
		return nil, nil, camera.NewUnsupportedImageTypeError(cde.imageType)
	}
}

func (cde *colorDepthExtrinsics) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "align::colorDepthExtrinsics::NextPointCloud")
	defer span.End()
	if cde.projector == nil {
		return nil, transform.NewNoIntrinsicsError("")
	}
	col, dm := camera.SimultaneousColorDepthNext(ctx, cde.color, cde.depth)
	if col == nil {
		return nil, errors.Errorf("could not get color image from source camera %q for join_color_depth camera", cde.colorName)
	}
	if dm == nil {
		return nil, errors.Errorf("could not get depth image from source camera %q for join_color_depth camera", cde.depthName)
	}
	if cde.aligner == nil {
		return cde.projector.RGBDToPointCloud(rimage.ConvertImage(col), dm)
	}
	alignedColor, alignedDepth, err := cde.aligner.AlignColorAndDepthImage(rimage.ConvertImage(col), dm)
	if err != nil {
		return nil, err
	}
	return cde.projector.RGBDToPointCloud(alignedColor, alignedDepth)
}

func (cde *colorDepthExtrinsics) Close(ctx context.Context) error {
	return multierr.Combine(cde.color.Close(ctx), cde.depth.Close(ctx))
}
