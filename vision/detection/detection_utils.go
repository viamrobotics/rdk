package detection

import (
	"image"

	"go.viam.com/rdk/rimage"
)

func OverlayDetections(img image.Image, dets []*Detection) image.Image {
	rimg := rimage.ConvertImage(img)
	for _, det := range dets {
		drawBox(rimg, &det.BoundingBox)
	}
	return rimg
}

// drawBox draws a red box over the image
func drawBox(img *rimage.Image, rec *image.Rectangle) {
	x0, y0, x1, y1 := rec.Min.X, rec.Min.Y, rec.Max.X, rec.Max.Y
	horizontal(x0, y0, x1, img, rimage.Red)
	horizontal(x0, y1, x1, img, rimage.Red)
	vertical(x0, y0, y1, img, rimage.Red)
	vertical(x1, y0, y1, img, rimage.Red)
}

// horizontal draws a horizontal line
func horizontal(x0, y, x1 int, img *rimage.Image, col rimage.Color) {
	for ; x0 <= x1; x0++ {
		img.SetXY(x0, y, col)
	}
}

// vertical draws a veritcal line
func vertical(x, y0, y1 int, img *rimage.Image, col rimage.Color) {
	for ; y0 <= y1; y0++ {
		img.SetXY(x, y0, col)
	}
}
