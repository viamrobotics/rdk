// Package imagetransform defines cameras that apply transforms for images in an image transformation pipeline.
// They are not original generators of image, but require an image source in order to function.
// They are typically registered as cameras in the API.
package imagetransform

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"identity",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, err := camera.FromDependencies(deps, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera for identity (%s): %w", sourceName, err)
			}
			proj, _ := camera.GetProjector(ctx, nil, source)
			return camera.New(source, proj)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "identity",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&transformConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"rotate",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, err := camera.FromDependencies(deps, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera for rotate (%s): %w", sourceName, err)
			}
			imgSrc := &rotateSource{source, camera.StreamType(attrs.Stream)}
			proj, _ := camera.GetProjector(ctx, nil, source)
			return camera.New(imgSrc, proj)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "rotate",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&transformConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"resize",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, err := camera.FromDependencies(deps, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera for resize (%s): %w", sourceName, err)
			}

			width := attrs.Width
			if width == 0 {
				width = 800
			}
			height := attrs.Height
			if height == 0 {
				height = 640
			}

			imgSrc := gostream.ResizeImageSource{Src: source, Width: width, Height: height}
			proj, _ := camera.GetProjector(ctx, nil, nil) // camera parameters from source camera do not work for resized images
			return camera.New(imgSrc, proj)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "resize",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&transformConfig{})
}

// rotateSource is the source to be rotated and the kind of image type.
type rotateSource struct {
	original gostream.ImageSource
	stream   camera.StreamType
}

// Next rotates the 2D image depending on the stream type.
func (rs *rotateSource) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::imagetransform::rotate::Next")
	defer span.End()
	orig, release, err := rs.original.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	switch rs.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		return imaging.Rotate(orig, 180, color.Black), release, nil
	case camera.DepthStream:
		dm, err := rimage.ConvertImageToDepthMap(orig)
		if err != nil {
			return nil, nil, err
		}
		return dm.Rotate(180), release, nil
	default:
		return nil, nil, camera.NewUnsupportedStreamError(rs.stream)
	}
}

// Close TODO.
func (rs *rotateSource) Close(ctx context.Context) error {
	return utils.TryClose(ctx, rs.original)
}
