package inject

import (
	"context"
	"image"

	"github.com/edaniels/gostream"

	"go.viam.com/core/utils"
)

// ImageSource is an injected image source.
type ImageSource struct {
	gostream.ImageSource
	NextFunc  func(ctx context.Context) (image.Image, func(), error)
	CloseFunc func() error
}

// Next calls the injected Next or the real version.
func (is *ImageSource) Next(ctx context.Context) (image.Image, func(), error) {
	if is.NextFunc == nil {
		return is.ImageSource.Next(ctx)
	}
	return is.NextFunc(ctx)
}

// Close calls the injected Close or the real version.
func (is *ImageSource) Close() error {
	if is.CloseFunc == nil {
		return utils.TryClose(is.ImageSource)
	}
	return is.CloseFunc()
}
