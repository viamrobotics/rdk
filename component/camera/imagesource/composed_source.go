package imagesource

import (
	"context"
	"image"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/core/component/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterComponent(camera.Subtype, "depth_to_pretty", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		return newDepthToPretty(r, config)
	}})

	registry.RegisterComponent(camera.Subtype, "overlay", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		return newOverlay(r, config)
	}})
}

type overlaySource struct {
	source gostream.ImageSource
}

func (os *overlaySource) Close() error {
	return nil
}

func (os *overlaySource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := os.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	return ii.Overlay(), func() {}, nil
}

func newOverlay(r robot.Robot, config config.Component) (camera.Camera, error) {
	source, ok := r.CameraByName(config.Attributes.String("source"))
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	return &camera.ImageSource{ImageSource: &overlaySource{source}}, nil

}

type depthToPretty struct {
	source gostream.ImageSource
}

func (dtp *depthToPretty) Close() error {
	return nil
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
	return ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), func() {}, nil
}

func newDepthToPretty(r robot.Robot, config config.Component) (camera.Camera, error) {
	source, ok := r.CameraByName(config.Attributes.String("source"))
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	return &camera.ImageSource{ImageSource: &depthToPretty{source}}, nil
}
