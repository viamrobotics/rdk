package align

import (
	"context"
	"encoding/json"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

var extrinsicsModel = resource.DefaultModelFamily.WithModel("align_color_depth_extrinsics")
func init() {
	resource.RegisterComponent(camera.API, extrinsicsModel,
		resource.Registration[camera.Camera, *extrinsicsConfig]{
			Constructor: func(ctx context.Context, deps resource.Dependencies,
				conf resource.Config, logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*extrinsicsConfig](conf)
				if err != nil {
					return nil, err
				}
				colorName := newConf.Color
				color, err := camera.FromDependencies(deps, colorName)
				if err != nil {
					return nil, fmt.Errorf("no color camera (%s): %w", colorName, err)
				}

				depthName := newConf.Depth
				depth, err := camera.FromDependencies(deps, depthName)
				if err != nil {
					return nil, fmt.Errorf("no depth camera (%s): %w", depthName, err)
				}
				src, err := newColorDepthExtrinsics(ctx, color, depth, newConf, logger)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
			},
			AttributeMapConverter: func(attributes rdkutils.AttributeMap) (*extrinsicsConfig, error) {
				if !attributes.Has("camera_system") {
					return nil, errors.New("missing camera_system")
				}

				b, err := json.Marshal(attributes["camera_system"])
				if err != nil {
					return nil, err
				}
				matrices, err := transform.NewDepthColorIntrinsicsExtrinsicsFromBytes(b)
				if err != nil {
					return nil, err
				}
				if err := matrices.CheckValid(); err != nil {
					return nil, err
				}
				attributes["camera_system"] = matrices

				return resource.TransformAttributeMap[*extrinsicsConfig](attributes)
			},
		})
}

// extrinsicsConfig is the attribute struct for aligning.
type extrinsicsConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	IntrinsicExtrinsic   interface{}                        `json:"camera_system"`
	ImageType            string                             `json:"output_image_type"`
	Color                string                             `json:"color_camera_name"`
	Depth                string                             `json:"depth_camera_name"`
	Debug                bool                               `json:"debug,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

func (cfg *extrinsicsConfig) Validate(path string) ([]string, error) {
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
func newColorDepthExtrinsics(ctx context.Context, color, depth camera.VideoSource, conf *extrinsicsConfig, logger golog.Logger,
) (camera.VideoSource, error) {
	alignment, err := rdkutils.AssertType[*transform.DepthColorIntrinsicsExtrinsics](conf.IntrinsicExtrinsic)
	if err != nil {
		return nil, err
	}
	if conf.CameraParameters == nil {
		return nil, transform.ErrNoIntrinsics
	}
	if conf.CameraParameters.Height <= 0 || conf.CameraParameters.Width <= 0 {
		return nil, errors.Errorf(
			"colorDepthExtrinsics needs Width and Height fields set in intrinsic_parameters. Got illegal dimensions (%d, %d)",
			conf.CameraParameters.Width,
			conf.CameraParameters.Height,
		)
	}
	// get the projector for the alignment camera
	imgType := camera.ImageType(conf.ImageType)
	videoSrc := &colorDepthExtrinsics{
		color:     gostream.NewEmbeddedVideoStream(color),
		colorName: conf.Color,
		depth:     gostream.NewEmbeddedVideoStream(depth),
		depthName: conf.Depth,
		aligner:   alignment,
		projector: conf.CameraParameters,
		imageType: imgType,
		height:    conf.CameraParameters.Height,
		width:     conf.CameraParameters.Width,
		debug:     conf.Debug,
		logger:    logger,
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(conf.CameraParameters, conf.DistortionParameters)
	return camera.NewVideoSourceFromReader(
		ctx,
		videoSrc,
		&cameraModel,
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
