package gostream

import (
	"context"
	"image"

	"github.com/disintegration/imaging"
	"github.com/pion/mediadevices/pkg/prop"
	"go.uber.org/multierr"
)

type resizeVideoSource struct {
	src           VideoSource
	stream        VideoStream
	width, height int
}

// NewResizeVideoSource returns a source that resizes images to the set dimensions.
func NewResizeVideoSource(src VideoSource, width, height int) VideoSource {
	rvs := &resizeVideoSource{
		src:    src,
		stream: NewEmbeddedVideoStream(src),
		width:  width,
		height: height,
	}
	return NewVideoSource(rvs, prop.Video{
		Width:  rvs.width,
		Height: rvs.height,
	})
}

// Read returns a resized image to Width x Height dimensions.
func (rvs resizeVideoSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, release, err := rvs.stream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	if release != nil {
		defer release()
	}

	return imaging.Resize(img, rvs.width, rvs.height, imaging.NearestNeighbor), func() {}, nil
}

// Close closes the underlying source.
func (rvs resizeVideoSource) Close(ctx context.Context) error {
	return multierr.Combine(rvs.stream.Close(ctx), rvs.src.Close(ctx))
}
