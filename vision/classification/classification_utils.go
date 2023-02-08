package classification

import (
	"fmt"
	"image"
	"image/color"

	"github.com/fogleman/gg"
	"go.viam.com/rdk/rimage"
)

// Overlay returns a color image with the classification labels and confidence scores overlaid on
// the original image.
func Overlay(img image.Image, classifications Classifications) (image.Image, error) {
	gimg := gg.NewContextForImage(img)
	for _, classification := range classifications {
		// TODO: figure out where on the image the classifications should go if there are multiple
		rimage.DrawString(gimg, fmt.Sprintf("%v: %v", classification.Label(), classification.Score()), image.Point{30, 30}, color.NRGBA{255, 0, 0, 255}, 30)
	}
	return gimg.Image(), nil
}
