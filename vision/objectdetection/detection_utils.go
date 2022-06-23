package objectdetection

import (
	"fmt"
	"image"
	"image/color"

	"github.com/fogleman/gg"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// Overlay returns a color image with the bounding boxes overlaid on the original image.
func Overlay(img image.Image, dets []Detection) (image.Image, error) {
	gimg := gg.NewContextForImage(img)
	for _, det := range dets {
		if !det.BoundingBox().In(img.Bounds()) {
			return nil, errors.Errorf("bounding box (%v) does not fit in image (%v)", det.BoundingBox(), img.Bounds())
		}
		drawDetection(gimg, det)
	}
	return gimg.Image(), nil
}

// drawDetection overlays text of the image label and score in the upper left hand of the bounding box
func drawDetection(img *gg.Context, d Detection) {
	red := &color.NRGBA{255, 0, 0, 255}
	box := d.BoundingBox()
	rimage.DrawRectangleEmpty(img, *box, red, 2.0)
	text := fmt.Sprintf("%s: %.2f", d.Label(), d.Score())
	rimage.DrawString(img, text, image.Point{box.Min.X, box.Min.Y}, red, 30)
}

// OverlayText writes a string in the top of the image.
func OverlayText(img image.Image, text string) image.Image {
	rimg := rimage.ConvertToImageWithDepth(img)
	gimg := gg.NewContextForImage(rimg.Color)
	rimage.DrawString(gimg, text, image.Point{30, 30}, color.NRGBA{255, 0, 0, 255}, 30)
	rimg.Color = rimage.ConvertImage(gimg.Image())
	return rimg
}

// drawBox draws a red box over the image, each side is 3 pixels wide.
func drawBox(img *rimage.Image, rec *image.Rectangle) {
	x0, y0, x1, y1 := rec.Min.X, rec.Min.Y, rec.Max.X, rec.Max.Y
	if x1 == img.Bounds().Dx() {
		x1--
	}
	if y1 == img.Bounds().Dy() {
		y1--
	}
	horizontal(x0, y0, x1, img, rimage.Red)
	horizontal(x0, y1, x1, img, rimage.Red)
	vertical(x0, y0, y1, img, rimage.Red)
	vertical(x1, y0, y1, img, rimage.Red)
}

// horizontal draws a horizontal line 3 pixels wide.
func horizontal(x0, y, x1 int, img *rimage.Image, col rimage.Color) {
	for x := x0; x <= x1; x++ {
		img.SetXY(x, y, col)
	}
	if y-1 >= 0 {
		for x := x0; x <= x1; x++ {
			img.SetXY(x, y-1, col)
		}
	}
	if y+1 < img.Height() {
		for x := x0; x <= x1; x++ {
			img.SetXY(x, y+1, col)
		}
	}
}

// vertical draws a veritcal line 3 pixels wide.
func vertical(x, y0, y1 int, img *rimage.Image, col rimage.Color) {
	for y := y0; y <= y1; y++ {
		img.SetXY(x, y, col)
	}
	if x-1 >= 0 {
		for y := y0; y <= y1; y++ {
			img.SetXY(x-1, y, col)
		}
	}
	if x+1 < img.Width() {
		for y := y0; y <= y1; y++ {
			img.SetXY(x+1, y, col)
		}
	}
}
