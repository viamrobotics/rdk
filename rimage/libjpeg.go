//go:build !no_cgo

package rimage

import (
	"image"
	"io"

	libjpeg "github.com/viam-labs/go-libjpeg/jpeg"
)

var jpegEncoderOptions = &libjpeg.EncoderOptions{Quality: 75, DCTMethod: libjpeg.DCTIFast}

// EncodeJPEG encode an image.Image in JPEG using libjpeg.
func EncodeJPEG(w io.Writer, src image.Image) error {
	switch v := src.(type) {
	case *Image:
		imgRGBA := image.NewRGBA(src.Bounds())
		ConvertToRGBA(imgRGBA, v)
		return libjpeg.Encode(w, imgRGBA, jpegEncoderOptions)
	default:
		return libjpeg.Encode(w, src, jpegEncoderOptions)
	}
}

// DecodeJPEG decode JPEG []bytes into an image.Image using libjpeg.
func DecodeJPEG(r io.Reader) (img image.Image, err error) {
	return libjpeg.Decode(r, &libjpeg.DecoderOptions{DCTMethod: libjpeg.DCTIFast})
}
