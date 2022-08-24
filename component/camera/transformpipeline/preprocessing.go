package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// preprocessDepthTransform applies pre-processing functions to depth maps in order to smooth edges and fill holes.
type preprocessDepthTransform struct {
	source gostream.ImageSource
}

func newDepthPreprocessTransform(source gostream.ImageSource) (gostream.ImageSource, error) {
	return &preprocessDepthTransform{source}, nil
}

// Next applies depth preprocessing to the next image.
func (os *preprocessDepthTransform) Next(ctx context.Context) (image.Image, func(), error) {
	i, release, err := os.source.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(i)
	if err != nil {
		return nil, nil, errors.Wrap(err, "transform source does not provide depth image")
	}
	dm, err = rimage.PreprocessDepthMap(dm, nil)
	if err != nil {
		return nil, nil, err
	}
	return dm, release, nil
}
