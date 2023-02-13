package classification

import (
	"fmt"
	"image"
	"image/color"

	"github.com/fogleman/gg"

	"go.viam.com/rdk/rimage"
)

// Overlay returns a color image with the classification label and confidence score overlaid on
// the original image.
func Overlay(img image.Image, label string, confidenceScore float64) (image.Image, error) {
	gimg := gg.NewContextForImage(img)
	rimage.DrawString(gimg, fmt.Sprintf("%v: %.2f %%", label, confidenceScore*100.), image.Point{30, 30}, color.NRGBA{255, 0, 0, 255}, 30)
	return gimg.Image(), nil
}
