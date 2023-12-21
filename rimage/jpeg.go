//go:build no_cgo

package rimage

import (
	"image"
	"image/jpeg"
	"io"
)

// EncodeJPEG encode an image.Image in JPEG.
func EncodeJPEG(w io.Writer, src image.Image) error {
	switch v := src.(type) {
	case *Image:
		imgRGBA := image.NewRGBA(src.Bounds())
		ConvertToRGBA(imgRGBA, v)
		return jpeg.Encode(w, imgRGBA, nil)
	default:
		return jpeg.Encode(w, src, nil)
	}
}

// DecodeJPEG decode JPEG []bytes into an image.Image using libjpeg.
func DecodeJPEG(r io.Reader) (img image.Image, err error) {
	return jpeg.Decode(r)
}
