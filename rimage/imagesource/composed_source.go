package imagesource

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
)

func init() {
	api.RegisterCamera("depthToPretty", func(r api.Robot, config api.Component, logger golog.Logger) (gostream.ImageSource, error) {
		return newDepthToPretty(r, config)
	})

	api.RegisterCamera("overlay", func(r api.Robot, config api.Component, logger golog.Logger) (gostream.ImageSource, error) {
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
		return nil, nil, fmt.Errorf("no depth")
	}
	return ii.Overlay(), func() {}, nil
}

func newOverlay(r api.Robot, config api.Component) (gostream.ImageSource, error) {
	source := r.CameraByName(config.Attributes.GetString("source"))
	if source == nil {
		return nil, fmt.Errorf("cannot find source camera (%s)", config.Attributes.GetString("source"))
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
		return nil, nil, fmt.Errorf("no depth")
	}
	return ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), func() {}, nil
}

func newDepthToPretty(r api.Robot, config api.Component) (gostream.ImageSource, error) {
	source := r.CameraByName(config.Attributes.GetString("source"))
	if source == nil {
		return nil, fmt.Errorf("cannot find source camera (%s)", config.Attributes.GetString("source"))
	}
	return &depthToPretty{source}, nil

}
