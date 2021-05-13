package imagesource

import (
	"context"
	"errors"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/registry"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot"
)

func init() {
	registry.RegisterCamera("depthToPretty", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		return newDepthToPretty(r, config)
	})

	registry.RegisterCamera("overlay", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		return newOverlay(r, config)
	})
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

func newOverlay(r robot.Robot, config config.Component) (gostream.ImageSource, error) {
	source := r.CameraByName(config.Attributes.String("source"))
	if source == nil {
		return nil, fmt.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	return &overlaySource{source}, nil

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

func newDepthToPretty(r robot.Robot, config config.Component) (gostream.ImageSource, error) {
	source := r.CameraByName(config.Attributes.String("source"))
	if source == nil {
		return nil, fmt.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	return &depthToPretty{source}, nil

}
