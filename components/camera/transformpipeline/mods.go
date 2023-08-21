package transformpipeline

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"
	"golang.org/x/image/draw"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// rotateConfig are the attributes for a rotate transform.
type rotateConfig struct {
	Angle float64 `json:"angle_degs"`
}

// rotateSource is the source to be rotated and the kind of image type.
type rotateSource struct {
	originalStream gostream.VideoStream
	stream         camera.ImageType
	angle          float64
}

// newRotateTransform creates a new rotation transform.
func newRotateTransform(ctx context.Context, source gostream.VideoSource, stream camera.ImageType, am utils.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*rotateConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, errors.Wrap(err, "cannot parse rotate attribute map")
	}

	props, err := propsFromVideoSource(ctx, source)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

	if props.DistortionParams != nil {
		cameraModel.Distortion = props.DistortionParams
	}
	reader := &rotateSource{gostream.NewEmbeddedVideoStream(source), stream, conf.Angle}
	src, err := camera.NewVideoSourceFromReader(ctx, reader, &cameraModel, stream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, stream, err
}

// Read rotates the 2D image depending on the stream type.
func (rs *rotateSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::rotate::Read")
	defer span.End()
	orig, release, err := rs.originalStream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	switch rs.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		// imaging.Rotate rotates an image counter-clockwise but our rotate function rotates in the
		// clockwise direction. The angle is negated here for consistency.
		return imaging.Rotate(orig, -(rs.angle), color.Black), release, nil
	case camera.DepthStream:
		dm, err := rimage.ConvertImageToDepthMap(ctx, orig)
		if err != nil {
			return nil, nil, err
		}
		return dm.Rotate(int(rs.angle)), release, nil
	default:
		return nil, nil, camera.NewUnsupportedImageTypeError(rs.stream)
	}
}

// Close closes the original stream.
func (rs *rotateSource) Close(ctx context.Context) error {
	return rs.originalStream.Close(ctx)
}

// resizeConfig are the attributes for a resize transform.
type resizeConfig struct {
	Height int `json:"height_px"`
	Width  int `json:"width_px"`
}

type resizeSource struct {
	originalStream gostream.VideoStream
	stream         camera.ImageType
	height         int
	width          int
}

// newResizeTransform creates a new resize transform.
func newResizeTransform(
	ctx context.Context, source gostream.VideoSource, stream camera.ImageType, am utils.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*resizeConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	if conf.Width == 0 {
		return nil, camera.UnspecifiedStream, errors.New("new width for resize transform cannot be 0")
	}
	if conf.Height == 0 {
		return nil, camera.UnspecifiedStream, errors.New("new height for resize transform cannot be 0")
	}

	reader := &resizeSource{gostream.NewEmbeddedVideoStream(source), stream, conf.Height, conf.Width}
	src, err := camera.NewVideoSourceFromReader(ctx, reader, nil, stream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, stream, err
}

// Read resizes the 2D image depending on the stream type.
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
		return nil, nil, camera.NewUnsupportedImageTypeError(rs.stream)
	}
}

// Close closes the original stream.
func (rs *resizeSource) Close(ctx context.Context) error {
	return rs.originalStream.Close(ctx)
}

// cropConfig are the attributes for a crop transform.
type cropConfig struct {
	XMin int `json:"x_min_px"`
	YMin int `json:"y_min_px"`
	XMax int `json:"x_max_px"`
	YMax int `json:"y_max_px"`
}

type cropSource struct {
	originalStream gostream.VideoStream
	imgType        camera.ImageType
	cropWindow     image.Rectangle
}

// newCropTransform creates a new crop transform.
func newCropTransform(
	ctx context.Context, source gostream.VideoSource, stream camera.ImageType, am utils.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*cropConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	if conf.XMin < 0 || conf.YMin < 0 {
		return nil, camera.UnspecifiedStream, errors.New("cannot set x_min or y_min to a negative number")
	}
	if conf.XMin >= conf.XMax {
		return nil, camera.UnspecifiedStream, errors.New("cannot crop image to 0 width (x_min is >= x_max)")
	}
	if conf.YMin >= conf.YMax {
		return nil, camera.UnspecifiedStream, errors.New("cannot crop image to 0 height (y_min is >= y_max)")
	}
	cropRect := image.Rect(conf.XMin, conf.YMin, conf.XMax, conf.YMax)

	reader := &cropSource{gostream.NewEmbeddedVideoStream(source), stream, cropRect}
	src, err := camera.NewVideoSourceFromReader(ctx, reader, nil, stream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, stream, err
}

// Read crops the 2D image depending on the crop window.
func (cs *cropSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::crop::Read")
	defer span.End()
	orig, release, err := cs.originalStream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	switch cs.imgType {
	case camera.ColorStream, camera.UnspecifiedStream:
		newImg := imaging.Crop(orig, cs.cropWindow)
		if newImg.Bounds().Empty() {
			return nil, nil, errors.New("crop transform cropped image to 0 pixels")
		}
		return newImg, release, nil
	case camera.DepthStream:
		dm, err := rimage.ConvertImageToDepthMap(ctx, orig)
		if err != nil {
			return nil, nil, err
		}
		newImg := dm.SubImage(cs.cropWindow)
		if newImg.Bounds().Empty() {
			return nil, nil, errors.New("crop transform cropped image to 0 pixels")
		}
		return newImg, release, nil
	default:
		return nil, nil, camera.NewUnsupportedImageTypeError(cs.imgType)
	}
}

// Close closes the original stream.
func (cs *cropSource) Close(ctx context.Context) error {
	return cs.originalStream.Close(ctx)
}
