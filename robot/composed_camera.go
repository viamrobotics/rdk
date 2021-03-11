package robot

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
)

func newDepthComposed(r *Robot, config api.Component) (gostream.ImageSource, error) {
	color := r.CameraByName(config.Attributes["color"])
	if color == nil {
		return nil, fmt.Errorf("cannot find color camera (%s)", config.Attributes["color"])
	}
	depth := r.CameraByName(config.Attributes["depth"])
	if depth == nil {
		return nil, fmt.Errorf("cannot find depth camera (%s)", config.Attributes["depth"])
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

func newOverlay(r *Robot, config api.Component) (gostream.ImageSource, error) {
	source := r.CameraByName(config.Attributes["source"])
	if source == nil {
		return nil, fmt.Errorf("cannot find source camera (%s)", config.Attributes["source"])
	}
	return &overlaySource{source}, nil

}
