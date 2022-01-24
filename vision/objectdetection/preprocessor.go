package objectdetection

import (
	"image"

	"go.viam.com/rdk/rimage"
)

// Preprocessor will apply processing to an input image before feeding it into the detector.
type Preprocessor func(image.Image) image.Image

// RemoveBlue will set the blue channel to 0 in every picture.
func RemoveBlue() Preprocessor {
	return func(img image.Image) image.Image {
		rimg := rimage.NewImage(img.Bounds().Dx(), img.Bounds().Dy())
		for y := 0; y < rimg.Height(); y++ {
			for x := 0; x < rimg.Width(); x++ {
				c := img.At(x, y)
				r, g, _, _ := c.RGBA()
				rimg.SetXY(x, y, rimage.NewColor(uint8(r), uint8(g), 0))
			}
		}
		return rimg
	}
}
