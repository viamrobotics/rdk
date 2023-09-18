// Package align defines the camera models that are used to align a color camera's output with a depth camera's output,
// in order to make point clouds.
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

var joinModel = resource.DefaultModelFamily.WithModel("join_color_depth")

//nolint:dupl
func init() {
	resource.RegisterComponent(camera.API, joinModel,
		resource.Registration[camera.Camera, *joinConfig]{
			Constructor: func(ctx context.Context, deps resource.Dependencies,
				conf resource.Config, logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*joinConfig](conf)
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
				src, err := newJoinColorDepth(ctx, color, depth, newConf, logger)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src), nil
			},
		})
}

// joinConfig is the attribute struct for aligning.
type joinConfig struct {
	ImageType            string                             `json:"output_image_type"`
	Color                string                             `json:"color_camera_name"`
	Depth                string                             `json:"depth_camera_name"`
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	Debug                bool                               `json:"debug,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

func (cfg *joinConfig) Validate(path string) ([]string, error) {
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
	color, depth         gostream.VideoStream
	colorName, depthName string
	underlyingCamera     camera.VideoSource
	projector            transform.Projector
	imageType            camera.ImageType
	debug                bool
	logger               golog.Logger
}

// newJoinColorDepth creates a gostream.VideoSource that aligned color and depth channels.
func newJoinColorDepth(ctx context.Context, color, depth camera.VideoSource, conf *joinConfig, logger golog.Logger,
) (camera.VideoSource, error) {
	imgType := camera.ImageType(conf.ImageType)
	// get intrinsic parameters from config, or from the underlying camera
	var camParams *transform.PinholeCameraIntrinsics
	if conf.CameraParameters == nil {
		if imgType == camera.DepthStream {
			props, err := depth.Properties(ctx)
			if err == nil {
				camParams = props.IntrinsicParams
			}
		} else {
			props, err := color.Properties(ctx)
			if err == nil {
				camParams = props.IntrinsicParams
			}
		}
	} else {
		camParams = conf.CameraParameters
	}
	err := camParams.CheckValid()
	if err != nil {
		return nil, errors.Wrap(
			err,
			"the intrinsic_parameters field in attributes, or the intrinsics from the underlying camera, encountered an error",
		)
	}
	videoSrc := &joinColorDepth{
		colorName: conf.Color,
		depthName: conf.Depth,
		color:     gostream.NewEmbeddedVideoStream(color),
		depth:     gostream.NewEmbeddedVideoStream(depth),
		projector: camParams,
		imageType: imgType,
		debug:     conf.Debug,
		logger:    logger,
	}
	if conf.Color == conf.Depth { // store the underlying VideoSource for an Images call
		videoSrc.underlyingCamera = color
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(conf.CameraParameters, conf.DistortionParameters)
	return camera.NewVideoSourceFromReader(
		ctx,
		videoSrc,
		&cameraModel,
		imgType,
	)
}

// Read returns the next image from either the color or depth camera..
// imageType parameter will determine which channel gets streamed.
func (jcd *joinColorDepth) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "align::joinColorDepth::Read")
	defer span.End()
	switch jcd.imageType {
	case camera.ColorStream, camera.UnspecifiedStream:
		return jcd.color.Next(ctx)
	case camera.DepthStream:
		return jcd.depth.Next(ctx)
	default:
		return nil, nil, camera.NewUnsupportedImageTypeError(jcd.imageType)
	}
}

func (jcd *joinColorDepth) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "align::joinColorDepth::NextPointCloud")
	defer span.End()
	if jcd.projector == nil {
		return nil, transform.NewNoIntrinsicsError("no intrinsic_parameters in camera attributes")
	}
	if jcd.colorName == jcd.depthName {
		return jcd.nextPointCloudFromImages(ctx)
	}
	col, dm := camera.SimultaneousColorDepthNext(ctx, jcd.color, jcd.depth)
	if col == nil {
		return nil, errors.Errorf("could not get color image from source camera %q for join_color_depth camera", jcd.colorName)
	}
	if dm == nil {
		return nil, errors.Errorf("could not get depth image from source camera %q for join_color_depth camera", jcd.depthName)
	}
	return jcd.projector.RGBDToPointCloud(rimage.ConvertImage(col), dm)
}

func (jcd *joinColorDepth) nextPointCloudFromImages(ctx context.Context) (pointcloud.PointCloud, error) {
	imgs, _, err := jcd.underlyingCamera.Images(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "could not call Images on underlying camera %q", jcd.colorName)
	}
	var col *rimage.Image
	var dm *rimage.DepthMap
	for _, img := range imgs {
		if img.SourceName == "color" {
			col = rimage.ConvertImage(img.Image)
		}
		if img.SourceName == "depth" {
			dm, err = rimage.ConvertImageToDepthMap(ctx, img.Image)
			if err != nil {
				return nil, errors.Wrap(err, "image called 'depth' from Images not actually a depth map")
			}
		}
	}
	return jcd.projector.RGBDToPointCloud(col, dm)
}

func (jcd *joinColorDepth) Close(ctx context.Context) error {
	return multierr.Combine(jcd.color.Close(ctx), jcd.depth.Close(ctx))
}
