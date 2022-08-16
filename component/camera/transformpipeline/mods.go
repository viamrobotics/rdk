package transformpipeline

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	rdkutils "go.viam.com/rdk/utils"
)

// newIdentityTransform creates a new identity transform
func newIdentityTransform(ctx context.Context, source camera.Camera) (camera.Camera, error) {
	proj, _ := camera.GetProjector(ctx, nil, source)
	return camera.New(source, proj)
}

// rotateSource is the source to be rotated and the kind of image type.
type rotateSource struct {
	original gostream.ImageSource
	stream   camera.StreamType
}

// newRotateTransform creates a new rotation transform
func newRotateTransform(ctx context.Context, source camera.Camera, stream camera.StreamType) (camera.Camera, error) {
	imgSrc := &rotateSource{source, stream}
	proj, _ := camera.GetProjector(ctx, nil, source)
	return camera.New(imgSrc, proj)
}

// Next rotates the 2D image depending on the stream type.
func (rs *rotateSource) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::rotate::Next")
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

// Close closes the original stream.
func (rs *rotateSource) Close(ctx context.Context) error {
	return utils.TryClose(ctx, rs.original)
}

// resizeAttrs are the attributes for a resize transform
type resizeAttrs struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

// newResizeTransform creates a new resize transform
func newResizeTransform(
	ctx context.Context, source camera.Camera, stream camera.StreamType, am config.AttributeMap,
) (camera.Camera, error) {
	conf, err := config.TransformAttributeMapToStruct(&(resizeAttrs{}), am)
	if err != nil {
		return nil, err
	}
	attrs, ok := conf.(*resizeAttrs)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	if attrs.Width == 0 {
		return nil, errors.New("new width for resize transform cannot be 0")
	}
	if attrs.Height == 0 {
		return nil, errors.New("new height for resize transform cannot be 0")
	}

	imgSrc := gostream.ResizeImageSource{Src: source, Width: attrs.Width, Height: attrs.Height}
	return camera.New(imgSrc, nil)
}
