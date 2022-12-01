package transformpipeline

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/image/draw"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

// rotateSource is the source to be rotated and the kind of image type.
type rotateSource struct {
	originalStream gostream.VideoStream
	stream         camera.StreamType
}

// newRotateTransform creates a new rotation transform.
func newRotateTransform(ctx context.Context, source gostream.VideoSource, stream camera.StreamType,
) (gostream.VideoSource, camera.StreamType, error) {
	var cameraModel *transform.PinholeCameraModel
	if cameraSrc, ok := source.(camera.Camera); ok {
		props, err := cameraSrc.Properties(ctx)
		if err != nil {
			return nil, camera.UnspecifiedStream, err
		}
		cameraModel = &transform.PinholeCameraModel{props.IntrinsicParams, props.DistortionParams}
	}
	reader := &rotateSource{gostream.NewEmbeddedVideoStream(source), stream}
	cam, err := camera.NewFromReader(ctx, reader, cameraModel, stream)
	return cam, stream, err
}

// Next rotates the 2D image depending on the stream type.
func (rs *rotateSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::rotate::Read")
	defer span.End()
	orig, release, err := rs.originalStream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	switch rs.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		return imaging.Rotate(orig, 180, color.Black), release, nil
	case camera.DepthStream:
		dm, err := rimage.ConvertImageToDepthMap(ctx, orig)
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
	return rs.originalStream.Close(ctx)
}

// resizeAttrs are the attributes for a resize transform.
type resizeAttrs struct {
	Height int `json:"height_px"`
	Width  int `json:"width_px"`
}

type resizeSource struct {
	originalStream gostream.VideoStream
	stream         camera.StreamType
	height         int
	width          int
}

// newResizeTransform creates a new resize transform.
func newResizeTransform(
	ctx context.Context, source gostream.VideoSource, stream camera.StreamType, am config.AttributeMap,
) (gostream.VideoSource, camera.StreamType, error) {
	conf, err := config.TransformAttributeMapToStruct(&(resizeAttrs{}), am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	attrs, ok := conf.(*resizeAttrs)
	if !ok {
		return nil, camera.UnspecifiedStream, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	if attrs.Width == 0 {
		return nil, camera.UnspecifiedStream, errors.New("new width for resize transform cannot be 0")
	}
	if attrs.Height == 0 {
		return nil, camera.UnspecifiedStream, errors.New("new height for resize transform cannot be 0")
	}

	reader := &resizeSource{gostream.NewEmbeddedVideoStream(source), stream, attrs.Height, attrs.Width}
	cam, err := camera.NewFromReader(ctx, reader, nil, stream)
	return cam, stream, err
}

// Next resizes the 2D image depending on the stream type.
func (rs *resizeSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::resize::Read")
	defer span.End()
	orig, release, err := rs.originalStream.Next(ctx)
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

// Close closes the original stream.
func (rs *resizeSource) Close(ctx context.Context) error {
	return rs.originalStream.Close(ctx)
}
