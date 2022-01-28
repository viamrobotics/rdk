package objectdetection

import (
	"image"

	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// Preprocessor will apply processing to an input image before feeding it into the detector.
type Preprocessor func(image.Image) image.Image

// ComposePreprocessors takes in a slice of Preprocessors and returns one Preprocessor function.
func ComposePreprocessors(pSlice []Preprocessor) Preprocessor {
	return func(img image.Image) image.Image {
		for _, p := range pSlice {
			img = p(img)
		}
		return img
	}
}

// RemoveColorChannel will set the requested channel color to 0 in every picture. only "R", "G", and "B" are allowed.
func RemoveColorChannel(col string) (Preprocessor, error) {
	switch col {
	case "R", "r":
		return func(img image.Image) image.Image {
			rimg := rimage.ConvertImage(img)
			for y := 0; y < rimg.Height(); y++ {
				for x := 0; x < rimg.Width(); x++ {
					c := rimg.GetXY(x, y)
					_, g, b := c.RGB255()
					rimg.SetXY(x, y, rimage.NewColor(0, g, b))
				}
			}
			return rimg
		}, nil
	case "G", "g":
		return func(img image.Image) image.Image {
			rimg := rimage.ConvertImage(img)
			for y := 0; y < rimg.Height(); y++ {
				for x := 0; x < rimg.Width(); x++ {
					c := rimg.GetXY(x, y)
					r, _, b := c.RGB255()
					rimg.SetXY(x, y, rimage.NewColor(r, 0, b))
				}
			}
			return rimg
		}, nil
	case "B", "b":
		return func(img image.Image) image.Image {
			rimg := rimage.ConvertImage(img)
			for y := 0; y < rimg.Height(); y++ {
				for x := 0; x < rimg.Width(); x++ {
					c := rimg.GetXY(x, y)
					r, g, _ := c.RGB255()
					rimg.SetXY(x, y, rimage.NewColor(r, g, 0))
				}
			}
			return rimg
		}, nil
	default:
		return nil, errors.Errorf("do not know channel %q, only valid channels are 'r', 'g', or 'b'", col)
	}
}
