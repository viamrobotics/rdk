package vision

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
)

func init() {
	api.RegisterCamera("depthComposed", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		return newDepthComposed(r, config)
	})

	api.RegisterCamera("overlay", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		return newOverlay(r, config)
	})

}

func newDepthComposed(r api.Robot, config api.Component) (gostream.ImageSource, error) {
	colorName := config.Attributes.GetString("color")
	color := r.CameraByName(colorName)
	if color == nil {
		return nil, fmt.Errorf("cannot find color camera (%s)", colorName)
	}

	depthName := config.Attributes.GetString("depth")
	depth := r.CameraByName(depthName)
	if depth == nil {
		return nil, fmt.Errorf("cannot find depth camera (%s)", depthName)
	}

	return rimage.NewDepthComposed(color, depth)
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
