package rimage

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmittmann/ppm"
	"go.uber.org/multierr"

	"go.viam.com/core/utils"
)

// readImageFromFile extracts the RGB, Z16, or "both" data from an image file.
// Aligned matters if you are reading a .both.gz file and both the rgb and d image are already aligned.
// Otherwise, if you are just reading an image, aligned is a moot parameter and should be false.
func readImageFromFile(path string, aligned bool) (image.Image, error) {
	if strings.HasSuffix(path, ".both.gz") {
		return ReadBothFromFile(path, aligned)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// NewImageFromFile returns an image read in from the given file.
func NewImageFromFile(fn string) (*Image, error) {
	img, err := readImageFromFile(fn, false) // extracting rgb, alignment doesn't matter
	if err != nil {
		return nil, err
	}

	return ConvertImage(img), nil
}

// WriteImageToFile writes the given image to a file at the supplied path.
func WriteImageToFile(path string, img image.Image) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, f.Close())
	}()

	switch filepath.Ext(path) {
	case ".png":
		return png.Encode(f, img)
	case ".ppm":
		return ppm.Encode(f, img)
	default:
		return fmt.Errorf("rimage.WriteImageToFile unsupported format: %s", filepath.Ext(path))
	}

}

// ConvertImage converts a go image into our Image type.
func ConvertImage(img image.Image) *Image {
	ii, ok := img.(*Image)
	if ok {
		return ii
	}

	iwd, ok := img.(*ImageWithDepth)
	if ok {
		return iwd.Color
	}

	b := img.Bounds()
	ii = NewImage(b.Max.X, b.Max.Y)

	switch orig := img.(type) {
	case *image.YCbCr:
		fastConvertYcbcr(ii, orig)
	case *image.RGBA:
		fastConvertRGBA(ii, orig)
	case *image.NRGBA:
		fastConvertNRGBA(ii, orig)
	default:
		for y := 0; y < ii.height; y++ {
			for x := 0; x < ii.width; x++ {
				ii.SetXY(x, y, NewColorFromColor(img.At(x, y)))
			}
		}
	}
	return ii
}

func fastConvertNRGBA(dst *Image, src *image.NRGBA) {
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {

			i := src.PixOffset(x, y)
			s := src.Pix[i : i+3 : i+3] // Small cap improves performance, see https://golang.org/issue/27857
			r, g, b := s[0], s[1], s[2]
			dst.SetXY(x, y, NewColor(r, g, b))
		}
	}

}

func fastConvertRGBA(dst *Image, src *image.RGBA) {
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {

			i := src.PixOffset(x, y)
			s := src.Pix[i : i+4 : i+4]
			r, g, b, a := s[0], s[1], s[2], s[3]

			if a == 255 {
				dst.SetXY(x, y, NewColor(r, g, b))
			} else {
				dst.SetXY(x, y, NewColorFromColor(color.RGBA{r, g, b, a}))
			}
		}
	}

}

func fastConvertYcbcr(dst *Image, src *image.YCbCr) {
	c := color.YCbCr{}

	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {

			yi := src.YOffset(x, y)
			ci := src.COffset(x, y)

			c.Y = src.Y[yi]
			c.Cb = src.Cb[ci]
			c.Cr = src.Cr[ci]

			r, g, b := color.YCbCrToRGB(c.Y, c.Cb, c.Cr)

			dst.SetXY(x, y, NewColor(r, g, b))
		}
	}

}

// IsImageFile returns if the given file is an image file based on what
// we support.
func IsImageFile(fn string) bool {
	extensions := []string{".both.gz", "ppm", "png", "jpg", "jpeg", "gif"}
	for _, suffix := range extensions {
		if strings.HasSuffix(fn, suffix) {
			return true
		}
	}
	return false
}
