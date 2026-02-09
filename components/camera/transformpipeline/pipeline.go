// Package transformpipeline defines image sources that apply transforms on images, and can be composed into
// an image transformation pipeline. The image sources are not original generators of image, but require an image source
// from a real camera or video in order to function.
package transformpipeline

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("transform")

func init() {
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, *transformConfig]{
			DeprecatedRobotConstructor: func(
				ctx context.Context,
				r any,
				conf resource.Config,
				logger logging.Logger,
			) (camera.Camera, error) {
				actualR, err := utils.AssertType[robot.Robot](r)
				if err != nil {
					return nil, err
				}
				newConf, err := resource.NativeConfig[*transformConfig](conf)
				if err != nil {
					return nil, err
				}
				sourceName := newConf.Source
				source, err := camera.FromProvider(actualR, sourceName)
				if err != nil {
					return nil, fmt.Errorf("no source camera for transform pipeline (%s): %w", sourceName, err)
				}
				src, err := newTransformPipeline(ctx, source, conf.ResourceName().AsNamed(), newConf, actualR, logger)
				if err != nil {
					return nil, err
				}
				return src, nil
			},
		})
}

// transformConfig specifies a stream and list of transforms to apply on images/pointclouds coming from a source camera.
type transformConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Source               string                             `json:"source"`
	Pipeline             []Transformation                   `json:"pipeline"`
}

// Validate ensures all parts of the config are valid.
func (cfg *transformConfig) Validate(path string) ([]string, []string, error) {
	var deps []string
	if len(cfg.Source) == 0 {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "source")
	}

	if cfg.CameraParameters != nil {
		if cfg.CameraParameters.Height < 0 || cfg.CameraParameters.Width < 0 {
			return nil, nil, errors.Errorf(
				"got illegal negative dimensions for width_px and height_px (%d, %d) fields set in intrinsic_parameters for transform camera",
				cfg.CameraParameters.Width, cfg.CameraParameters.Height,
			)
		}
	}

	deps = append(deps, cfg.Source)
	return deps, nil, nil
}

// transformCamera wraps a transform's read function into a full camera.Camera implementation.
type transformCamera struct {
	resource.Named
	resource.AlwaysRebuild
	readFunc    func(ctx context.Context) (image.Image, func(), error)
	stream      camera.ImageType
	cameraModel *transform.PinholeCameraModel
}

func newTransformCamera(
	readFunc func(ctx context.Context) (image.Image, func(), error),
	stream camera.ImageType,
	cameraModel *transform.PinholeCameraModel,
) camera.Camera {
	return &transformCamera{
		readFunc:    readFunc,
		stream:      stream,
		cameraModel: cameraModel,
	}
}

func (tc *transformCamera) Images(
	ctx context.Context,
	filterSourceNames []string,
	extra map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::transformCamera::Images")
	defer span.End()
	img, release, err := tc.readFunc(ctx)
	if err != nil {
		return nil, resource.ResponseMetadata{}, err
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	ts := time.Now()
	namedImg, err := camera.NamedImageFromImage(img, "", utils.MimeTypeJPEG, data.Annotations{})
	if err != nil {
		return nil, resource.ResponseMetadata{}, err
	}
	return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: ts}, nil
}

func (tc *transformCamera) NextPointCloud(
	ctx context.Context,
	extra map[string]interface{},
) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::transformCamera::NextPointCloud")
	defer span.End()
	if tc.cameraModel == nil || tc.cameraModel.PinholeCameraIntrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("cannot do a projection to a point cloud")
	}
	img, release, err := tc.readFunc(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	dm, err := rimage.ConvertImageToDepthMap(ctx, img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot project to a point cloud")
	}
	return depthadapter.ToPointCloud(dm, tc.cameraModel.PinholeCameraIntrinsics), nil
}

func (tc *transformCamera) Properties(ctx context.Context) (camera.Properties, error) {
	result := camera.Properties{}
	if tc.cameraModel == nil {
		return result, nil
	}
	if (tc.cameraModel.PinholeCameraIntrinsics != nil) && (tc.stream == camera.DepthStream) {
		result.SupportsPCD = true
	}
	result.ImageType = tc.stream
	result.IntrinsicParams = tc.cameraModel.PinholeCameraIntrinsics
	if tc.cameraModel.Distortion != nil {
		result.DistortionParams = tc.cameraModel.Distortion
	}
	return result, nil
}

func (tc *transformCamera) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (tc *transformCamera) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, resource.ErrDoUnimplemented
}

func (tc *transformCamera) Close(ctx context.Context) error {
	return nil
}

func newTransformPipeline(
	ctx context.Context,
	source camera.Camera,
	named resource.Named,
	cfg *transformConfig,
	r robot.Robot,
	logger logging.Logger,
) (camera.Camera, error) {
	if source == nil {
		return nil, errors.New("no source camera for transform pipeline")
	}
	if len(cfg.Pipeline) == 0 {
		return nil, errors.New("pipeline has no transforms in it")
	}
	// check if the source produces a depth image or color image
	img, err := camera.DecodeImageFromCamera(ctx, source, nil, nil)

	var streamType camera.ImageType
	if err != nil {
		streamType = camera.UnspecifiedStream
	} else if _, ok := img.(*rimage.DepthMap); ok {
		streamType = camera.DepthStream
	} else if _, ok := img.(*image.Gray16); ok {
		streamType = camera.DepthStream
	} else {
		streamType = camera.ColorStream
	}
	// loop through the pipeline and create the image flow
	pipeline := make([]camera.Camera, 0, len(cfg.Pipeline))
	lastSource := source
	for _, tr := range cfg.Pipeline {
		src, newStreamType, err := buildTransform(ctx, r, lastSource, streamType, tr)
		if err != nil {
			return nil, err
		}
		pipeline = append(pipeline, src)
		lastSource = src
		streamType = newStreamType
	}

	var cameraModel *transform.PinholeCameraModel
	if cfg.CameraParameters != nil || cfg.DistortionParameters != nil {
		cm := transform.PinholeCameraModel{}
		cm.PinholeCameraIntrinsics = cfg.CameraParameters
		if cfg.DistortionParameters != nil {
			cm.Distortion = cfg.DistortionParameters
		}
		cameraModel = &cm
	}

	return &transformPipeline{
		Named:       named,
		pipeline:    pipeline,
		src:         lastSource,
		cameraModel: cameraModel,
		stream:      streamType,
		logger:      logger,
	}, nil
}

type transformPipeline struct {
	resource.Named
	resource.AlwaysRebuild
	pipeline    []camera.Camera
	src         camera.Camera
	cameraModel *transform.PinholeCameraModel
	stream      camera.ImageType
	logger      logging.Logger
}

func (tp *transformPipeline) Images(
	ctx context.Context,
	filterSourceNames []string,
	extra map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::Images")
	defer span.End()
	return tp.src.Images(ctx, filterSourceNames, extra)
}

func (tp *transformPipeline) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::NextPointCloud")
	defer span.End()
	if lastElem, ok := tp.pipeline[len(tp.pipeline)-1].(camera.PointCloudSource); ok {
		pc, err := lastElem.NextPointCloud(ctx, extra)
		if err != nil {
			return nil, errors.Wrap(err, "function NextPointCloud not defined for last videosource in transform pipeline")
		}
		return pc, nil
	}
	return nil, errors.New("function NextPointCloud not defined for last videosource in transform pipeline")
}

func (tp *transformPipeline) Properties(ctx context.Context) (camera.Properties, error) {
	result := camera.Properties{}
	if tp.cameraModel == nil {
		return result, nil
	}
	if (tp.cameraModel.PinholeCameraIntrinsics != nil) && (tp.stream == camera.DepthStream) {
		result.SupportsPCD = true
	}
	result.ImageType = tp.stream
	result.IntrinsicParams = tp.cameraModel.PinholeCameraIntrinsics
	if tp.cameraModel.Distortion != nil {
		result.DistortionParams = tp.cameraModel.Distortion
	}
	return result, nil
}

func (tp *transformPipeline) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (tp *transformPipeline) Close(ctx context.Context) error {
	return nil
}
