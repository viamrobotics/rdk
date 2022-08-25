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
	"golang.org/x/image/draw"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	rdkutils "go.viam.com/rdk/utils"
)

// rotateSource is the source to be rotated and the kind of image type.
type rotateSource struct {
	original gostream.ImageSource
	stream   camera.StreamType
}

// newRotateTransform creates a new rotation transform.
func newRotateTransform(source gostream.ImageSource, stream camera.StreamType) (gostream.ImageSource, error) {
	return &rotateSource{source, stream}, nil
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

// resizeAttrs are the attributes for a resize transform.
type resizeAttrs struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type resizeSource struct {
	original gostream.ImageSource
	stream   camera.StreamType
	height   int
	width    int
}

// newResizeTransform creates a new resize transform.
func newResizeTransform(
	source gostream.ImageSource, stream camera.StreamType, am config.AttributeMap,
) (gostream.ImageSource, error) {
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

	return &resizeSource{source, stream, attrs.Height, attrs.Width}, nil
}

// Next resizes the 2D image depending on the stream type.
func (rs *resizeSource) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::resize::Next")
	defer span.End()
	orig, release, err := rs.original.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	switch rs.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		dst := image.NewRGBA(image.Rect(0, 0, rs.width, rs.height))
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), orig, orig.Bounds(), draw.Over, nil)
		return dst, release, nil
	case camera.DepthStream:
		dm, err := rimage.ConvertImageToGray16(orig)
		if err != nil {
			return nil, nil, err
		}
		dst := image.NewGray16(image.Rect(0, 0, rs.width, rs.height))
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), dm, dm.Bounds(), draw.Over, nil)
		return dst, release, nil
	default:
		return nil, nil, camera.NewUnsupportedStreamError(rs.stream)
	}
}
