package align

import (
	"context"
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
)

var homographyModel = resource.DefaultModelFamily.WithModel("align_color_depth_homography")

//nolint:dupl
func init() {
	resource.RegisterComponent(camera.API, homographyModel,
		resource.Registration[camera.Camera, *homographyConfig]{
			Constructor: func(ctx context.Context, deps resource.Dependencies,
				conf resource.Config, logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*homographyConfig](conf)
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
				src, err := newColorDepthHomography(ctx, color, depth, newConf, logger)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
			},
		})
}

// homographyConfig is the attribute struct for aligning.
type homographyConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	Homography           *transform.RawDepthColorHomography `json:"homography"`
	Color                string                             `json:"color_camera_name"`
	Depth                string                             `json:"depth_camera_name"`
	ImageType            string                             `json:"output_image_type"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Debug                bool                               `json:"debug,omitempty"`
}

func (cfg *homographyConfig) Validate(path string) ([]string, error) {
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

// colorDepthHomography takes a color and depth image source and aligns them together using homography.
type colorDepthHomography struct {
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

// newColorDepthHomography creates a gostream.VideoSource that aligned color and depth channels.
func newColorDepthHomography(ctx context.Context, color, depth camera.VideoSource, conf *homographyConfig, logger golog.Logger,
) (camera.VideoSource, error) {
	if conf.Homography == nil {
		return nil, errors.New("homography field in attributes cannot be empty")
	}
	if conf.CameraParameters == nil {
		return nil, errors.Wrap(transform.ErrNoIntrinsics, "intrinsic_parameters field in attributes cannot be empty")
	}
	if conf.CameraParameters.Height <= 0 || conf.CameraParameters.Width <= 0 {
		return nil, errors.Errorf(
			"colorDepthHomography needs Width and Height fields set in intrinsic_parameters. Got illegal dimensions (%d, %d)",
			conf.CameraParameters.Width,
			conf.CameraParameters.Height,
		)
	}
	homography, err := transform.NewDepthColorHomography(conf.Homography)
	if err != nil {
		return nil, err
	}
	imgType := camera.ImageType(conf.ImageType)

	videoSrc := &colorDepthHomography{
		color:     gostream.NewEmbeddedVideoStream(color),
		colorName: conf.Color,
		depth:     gostream.NewEmbeddedVideoStream(depth),
		depthName: conf.Depth,
		aligner:   homography,
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
// imageType parameter will determine which channel gets streamed.
func (acd *colorDepthHomography) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "align::colorDepthHomography::Read")
	defer span.End()
	switch acd.imageType {
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
		return nil, nil, camera.NewUnsupportedImageTypeError(acd.imageType)
	}
}

func (acd *colorDepthHomography) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "align::colorDepthHomography::NextPointCloud")
	defer span.End()
	if acd.projector == nil {
		return nil, transform.NewNoIntrinsicsError("")
	}
	col, dm := camera.SimultaneousColorDepthNext(ctx, acd.color, acd.depth)
	if col == nil {
		return nil, errors.Errorf("could not get color image from source camera %q for join_color_depth camera", acd.colorName)
	}
	if dm == nil {
		return nil, errors.Errorf("could not get depth image from source camera %q for join_color_depth camera", acd.depthName)
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

func (acd *colorDepthHomography) Close(ctx context.Context) error {
	return multierr.Combine(acd.color.Close(ctx), acd.depth.Close(ctx))
}
