package transformpipeline

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/image/draw"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// rotateConfig are the attributes for a rotate transform.
type rotateConfig struct {
	Angle float64 `json:"angle_degs"`
}

// rotateSource is the source to be rotated and the kind of image type.
type rotateSource struct {
	src    camera.VideoSource
	stream camera.ImageType
	angle  float64
}

// newRotateTransform creates a new rotation transform.
func newRotateTransform(ctx context.Context, source camera.VideoSource, stream camera.ImageType, am utils.AttributeMap,
) (camera.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*rotateConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, errors.Wrap(err, "cannot parse rotate attribute map")
	}

	if !am.Has("angle_degs") {
		conf.Angle = 180 // Default to 180 for backwards-compatibility
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
	reader := &rotateSource{source, stream, conf.Angle}
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
	orig, release, err := camera.ReadImage(ctx, rs.src)
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

func (rs *rotateSource) Close(ctx context.Context) error {
	return nil
}

// resizeConfig are the attributes for a resize transform.
type resizeConfig struct {
	Height int `json:"height_px"`
	Width  int `json:"width_px"`
}

type resizeSource struct {
	src    camera.VideoSource
	stream camera.ImageType
	height int
	width  int
}

// newResizeTransform creates a new resize transform.
func newResizeTransform(
	ctx context.Context, source camera.VideoSource, stream camera.ImageType, am utils.AttributeMap,
) (camera.VideoSource, camera.ImageType, error) {
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

	reader := &resizeSource{source, stream, conf.Height, conf.Width}
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
	orig, release, err := camera.ReadImage(ctx, rs.src)
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

func (rs *resizeSource) Close(ctx context.Context) error {
	return nil
}

// cropConfig are the attributes for a crop transform.
type cropConfig struct {
	XMin        float64 `json:"x_min_px"`
	YMin        float64 `json:"y_min_px"`
	XMax        float64 `json:"x_max_px"`
	YMax        float64 `json:"y_max_px"`
	ShowCropBox bool    `json:"overlay_crop_box"`
}

type cropSource struct {
	src         camera.VideoSource
	imgType     camera.ImageType
	cropWindow  image.Rectangle
	cropRel     []float64
	showCropBox bool
	imgBounds   image.Rectangle
}

// newCropTransform creates a new crop transform.
func newCropTransform(
	ctx context.Context, source camera.VideoSource, stream camera.ImageType, am utils.AttributeMap,
) (camera.VideoSource, camera.ImageType, error) {
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
	cropRect := image.Rectangle{}
	cropRel := []float64{}
	switch {
	case conf.XMax == 1.0 && conf.YMax == 1.0 && conf.XMin == 0.0 && conf.YMin == 0.0:
		// interpreting this to mean cropping to the upper left pixel
		// you wouldn't use crop if you weren't gonna crop your image
		cropRect = image.Rect(0, 0, 1, 1)
	case conf.XMax > 1.0 || conf.YMax > 1.0:
		// you're not using relative boundaries if either max value is greater than 1
		cropRect = image.Rect(int(conf.XMin), int(conf.YMin), int(conf.XMax), int(conf.YMax))
	default:
		// everything else assumes relative boundaries
		if conf.XMin > 1.0 || conf.YMin > 1.0 { // but rel values cannot be greater than 1.0
			return nil, camera.UnspecifiedStream,
				errors.New("if using relative bounds between 0 and 1 for cropping, all crop attributes must be between 0 and 1")
		}
		cropRel = []float64{conf.XMin, conf.YMin, conf.XMax, conf.YMax}
	}

	reader := &cropSource{
		src:         source,
		imgType:     stream,
		cropWindow:  cropRect,
		cropRel:     cropRel,
		showCropBox: conf.ShowCropBox,
	}
	src, err := camera.NewVideoSourceFromReader(ctx, reader, nil, stream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, stream, err
}

func (cs *cropSource) relToAbsCrop(img image.Image) image.Rectangle {
	xMin, yMin, xMax, yMax := cs.cropRel[0], cs.cropRel[1], cs.cropRel[2], cs.cropRel[3]
	// Get image bounds
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Convert relative coordinates to absolute pixels
	x1 := bounds.Min.X + int(xMin*float64(width))
	y1 := bounds.Min.Y + int(yMin*float64(height))
	x2 := bounds.Min.X + int(xMax*float64(width))
	y2 := bounds.Min.Y + int(yMax*float64(height))

	// Create cropping rectangle
	rect := image.Rect(x1, y1, x2, y2)
	return rect
}

// Read crops the 2D image depending on the crop window.
func (cs *cropSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::crop::Read")
	defer span.End()
	orig, release, err := camera.ReadImage(ctx, cs.src)
	if err != nil {
		return nil, nil, err
	}
	if cs.imgBounds.Empty() {
		cs.imgBounds = orig.Bounds()
	}
	// check to see if the image size changed, and if the relative crop needs to be redone
	if cs.imgBounds != orig.Bounds() && len(cs.cropRel) != 0 {
		cs.cropWindow = image.Rectangle{} // reset the crop box
	}
	if cs.cropWindow.Empty() && len(cs.cropRel) != 0 {
		cs.cropWindow = cs.relToAbsCrop(orig)
	}
	switch cs.imgType {
	case camera.ColorStream, camera.UnspecifiedStream:
		if cs.showCropBox {
			newDet := objectdetection.NewDetection(cs.cropWindow, 1.0, "crop")
			dets := []objectdetection.Detection{newDet}
			newImg, err := objectdetection.Overlay(orig, dets)
			if err != nil {
				return nil, nil, fmt.Errorf("could not overlay crop box: %w", err)
			}
			return newImg, release, nil
		} else {
			newImg := imaging.Crop(orig, cs.cropWindow)
			if newImg.Bounds().Empty() {
				return nil, nil, errors.New("crop transform cropped image to 0 pixels")
			}
			return newImg, release, nil
		}
	case camera.DepthStream:
		if cs.showCropBox {
			return nil, nil, errors.New("crop box overlay not supported for depth images")
		}
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

func (cs *cropSource) Close(ctx context.Context) error {
	return nil
}
