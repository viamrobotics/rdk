package imagetransform

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
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
			attrs, ok := config.ConvertedAttributes.(*camera.TransformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newDepthToPretty(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "depth_to_pretty",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.TransformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.TransformConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"overlay",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.TransformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newOverlay(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "overlay",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.TransformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.TransformConfig{})
}

type overlaySource struct {
	colorSource camera.Camera
	depthSource camera.Camera
}

func (os *overlaySource) Next(ctx context.Context) (image.Image, func(), error) {
	col, dm := camera.SimultaneousColorDepthRequest(ctx, os.colorSource, os.depthSource)
	if col == nil || dm == nil {
		return nil, nil, errors.New("requested color or depth image from camera is nil")
	}
	return rimage.Overlay(rimage.ConvertImage(col), dm), func() {}, nil
}

func newOverlay(ctx context.Context, deps registry.Dependencies, attrs *camera.TransformConfig) (camera.Camera, error) {
	colorSource, err := camera.FromDependencies(deps, attrs.ColorSource)
	if err != nil {
		return nil, fmt.Errorf("no color source camera (%s): %w", attrs.ColorSource, err)
	}
	depthSource, err := camera.FromDependencies(deps, attrs.DepthSource)
	if err != nil {
		return nil, fmt.Errorf("no depth source camera (%s): %w", attrs.DepthSource, err)
	}
	imgSrc := &overlaySource{colorSource, depthSource}
	proj, _ := camera.GetProjector(ctx, attrs, colorSource)
	return camera.New(imgSrc, proj)
}

type depthToPretty struct {
	source gostream.ImageSource
}

func (dtp *depthToPretty) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := dtp.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	return rimage.MakeImageWithDepth(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), ii.Depth, true), func() {}, nil
}

func newDepthToPretty(ctx context.Context, deps registry.Dependencies, attrs *camera.AttrConfig) (camera.Camera, error) {
	source, err := camera.FromDependencies(deps, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	imgSrc := &depthToPretty{source}
	proj, _ := camera.GetProjector(ctx, attrs, source)
	return camera.New(imgSrc, proj)
}
