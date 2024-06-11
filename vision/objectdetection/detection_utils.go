package objectdetection

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/fogleman/gg"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// Overlay returns a color image with the bounding boxes overlaid on the original image.
func Overlay(img image.Image, dets []Detection) (image.Image, error) {
	bounds := img.Bounds()
	boxOverlay := gg.NewContext(bounds.Dx(), bounds.Dy())
	for _, det := range dets {
		if !det.BoundingBox().In(bounds) {
			return nil, errors.Errorf("bounding box (%v) does not fit in image (%v)", det.BoundingBox(), bounds)
		}
		drawDetection(boxOverlay, det)
	}
	overlayImg := boxOverlay.Image()
	resultImg := image.NewNRGBA(bounds) // to keep the original image intact
	draw.Draw(resultImg, bounds, img, image.Point{}, draw.Src)
	draw.DrawMask(resultImg, bounds, overlayImg, image.Point{}, overlayImg, image.Point{}, draw.Over)
	return resultImg, nil
}

// drawDetection overlays text of the image label and score in the upper left hand of the bounding box.
func drawDetection(img *gg.Context, d Detection) {
	red := &color.NRGBA{255, 0, 0, 255}
	box := d.BoundingBox()
	rimage.DrawRectangleEmpty(img, *box, red, 2.0)
	text := fmt.Sprintf("%s: %.2f", d.Label(), d.Score())
	rimage.DrawString(img, text, image.Point{box.Min.X, box.Min.Y}, red, 30)
}

// OverlayText writes a string in the top of the image.
func OverlayText(img image.Image, text string) image.Image {
	gimg := gg.NewContextForImage(img)
	rimage.DrawString(gimg, text, image.Point{30, 30}, color.NRGBA{255, 0, 0, 255}, 30)
	return gimg.Image()
}
