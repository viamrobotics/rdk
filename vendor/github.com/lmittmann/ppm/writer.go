package ppm

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
)

var errUnsupportedColorMode = errors.New("ppm: color mode not supported")

// Encode writes the Image img to Writer w in PPM format.
func Encode(w io.Writer, img image.Image) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	switch img.ColorModel() {
	case color.RGBAModel:
		if rgbaImg, ok := img.(*image.RGBA); ok {
			return encodeRGBA(bw, rgbaImg)
		}
		return encodeRGBAImage(bw, img)
	default:
		return errUnsupportedColorMode
	}
}

func encodeRGBAImage(w io.Writer, img image.Image) (err error) {
	rec := img.Bounds()

	// write header
	if _, err = fmt.Fprintf(w, "P6\n%d %d\n255\n", rec.Dx(), rec.Dy()); err != nil {
		return
	}

	// write pixels
	pixel := make([]byte, 3)
	var c color.RGBA
	for y := rec.Min.Y; y < rec.Max.Y; y++ {
		for x := rec.Min.X; x < rec.Max.X; x++ {
			c = img.At(x, y).(color.RGBA)
			pixel[0], pixel[1], pixel[2] = c.R, c.G, c.B
			if _, err = w.Write(pixel); err != nil {
				return
			}
		}
	}
	return
}

func encodeRGBA(w io.Writer, img *image.RGBA) (err error) {
	// write header
	if _, err = fmt.Fprintf(w, "P6\n%d %d\n255\n", img.Rect.Dx(), img.Rect.Dy()); err != nil {
		return
	}

	// write pixels
	for i := range img.Pix {
		if i%4 == 0 {
			if _, err = w.Write(img.Pix[i : i+3]); err != nil {
				return
			}
		}
	}
	return
}
