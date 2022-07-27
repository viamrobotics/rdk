package imagetransform

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"depth_to_pretty",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newDepthToPretty(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "depth_to_pretty",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &transformConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"overlay",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newOverlay(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "overlay",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &transformConfig{})
}

type overlaySource struct {
	src  camera.Camera
	proj rimage.Projector // keep a local copy for faster use
}

func (os *overlaySource) Next(ctx context.Context) (image.Image, func(), error) {
	if os.proj == nil {
		return nil, nil, transform.ErrNoIntrinsics
	}
	pc, err := os.src.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	col, dm, err := os.proj.PointCloudToRGBD(pc)
	if err != nil {
		return nil, nil, err
	}
	return rimage.Overlay(col, dm), func() {}, nil
}

func (os *overlaySource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return os.src.NextPointCloud(ctx)
}

func newOverlay(ctx context.Context, deps registry.Dependencies, attrs *transformConfig) (camera.Camera, error) {
	source, err := camera.FromDependencies(deps, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	proj, _ := camera.GetProjector(ctx, nil, source)
	imgSrc := &overlaySource{source, proj}
	return camera.New(imgSrc, proj)
}

type depthToPretty struct {
	source camera.Camera
	proj   rimage.Projector // keep a local copy for faster use
}

func (dtp *depthToPretty) Next(ctx context.Context) (image.Image, func(), error) {
	i, release, err := dtp.source.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(i)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "source camera does not make depth maps")
	}
	return dm.ToPrettyPicture(0, rimage.MaxDepth), release, nil
}

func (dtp *depthToPretty) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if dtp.proj == nil {
		return nil, transform.ErrNoIntrinsics
	}
	// get the original depth map and colorful output
	col, dm := camera.SimultaneousColorDepthNext(ctx, dtp, dtp.source)
	if col == nil || dm == nil {
		return nil, errors.New("requested color or depth image from camera is nil")
	}
	return dtp.proj.RGBDToPointCloud(rimage.ConvertImage(col), dm)
}

func newDepthToPretty(ctx context.Context, deps registry.Dependencies, attrs *transformConfig) (camera.Camera, error) {
	source, err := camera.FromDependencies(deps, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	proj, _ := camera.GetProjector(ctx, nil, source)
	imgSrc := &depthToPretty{source, proj}
	return camera.New(imgSrc, proj)
}
