package rimage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/multierr"

	"go.viam.com/rdk/logging"
)

// LazyEncodedImage defers the decoding of an image until necessary.
type LazyEncodedImage struct {
	imgBytes []byte
	mimeType string

	decodeOnce     sync.Once
	decodeImageErr interface{}
	decodedImage   image.Image

	decodeConfigOnce sync.Once
	decodeConfigErr  interface{}
	bounds           *image.Rectangle
	colorModel       color.Model
}

// NewLazyEncodedImage returns a new image that will only get decoded once actual data is needed
// from it. This is helpful for zero copy scenarios. If a width or height of the image is unknown,
// pass 0 or -1; when done a decode will happen on Bounds. In the future this can probably go
// away with reading all metadata from the header of the image bytes.
// NOTE: It is recommended to call one of the Decode* methods and check for errors before using
// image.Image methods (Bounds, ColorModel, or At) to avoid panics.
func NewLazyEncodedImage(imgBytes []byte, mimeType string) image.Image {
	if mimeType == "" {
		logging.Global().Warn("NewLazyEncodedImage called without a mime_type. " +
			"Sniffing bytes to detect mime_type. Specify mime_type to reduce CPU utilization")
		mimeType = http.DetectContentType(imgBytes)
	}

	if !strings.HasPrefix(mimeType, "image/") {
		logging.Global().Warnf("NewLazyEncodedImage resolving to non image mime_type: %s", mimeType)
	}

	return &LazyEncodedImage{
		imgBytes: imgBytes,
		mimeType: mimeType,
	}
}

// Helper method for checking lei errors.
func (lei *LazyEncodedImage) checkError(err interface{}) error {
	if err != nil {
		switch e := err.(type) {
		case error:
			return e
		default:
			return fmt.Errorf("%v", err)
		}
	}
	return nil
}

// DecodeImage decodes the image. Returns nil if no errors occurred.
// This method is idempotent.
func (lei *LazyEncodedImage) DecodeImage() error {
	lei.decodeOnce.Do(func() {
		defer func() {
			if err := recover(); err != nil {
				lei.decodeImageErr = err
			}
		}()
		lei.decodedImage, lei.decodeImageErr = DecodeImage(
			context.Background(),
			lei.imgBytes,
			lei.mimeType,
		)
	})
	return lei.checkError(lei.decodeImageErr)
}

// DecodeConfig decodes the image configuration. Returns nil if no errors occurred.
// This method is idempotent.
func (lei *LazyEncodedImage) DecodeConfig() error {
	lei.decodeConfigOnce.Do(func() {
		defer func() {
			if err := recover(); err != nil {
				lei.decodeConfigErr = err
			}
		}()
		reader := bytes.NewReader(lei.imgBytes)
		header, _, err := image.DecodeConfig(reader)
		lei.decodeConfigErr = err
		if err == nil {
			lei.bounds = &image.Rectangle{image.Point{0, 0}, image.Point{header.Width, header.Height}}
			lei.colorModel = header.ColorModel
		}
	})
	return lei.checkError(lei.decodeConfigErr)
}

// DecodeAll decodes the image and its configuration. Returns nil if no errors occurred.
// This method is idempotent.
func (lei *LazyEncodedImage) DecodeAll() error {
	configErr := lei.DecodeConfig()
	decodeErr := lei.DecodeImage()
	if configErr != nil || decodeErr != nil {
		return multierr.Combine(configErr, decodeErr)
	}
	return nil
}

// MIMEType returns the encoded Image's MIME type.
func (lei *LazyEncodedImage) MIMEType() string {
	return lei.mimeType
}

// RawData returns the encoded Image's raw data.
// Note: This is not a copy and should only be read from.
func (lei *LazyEncodedImage) RawData() []byte {
	return lei.imgBytes
}

// DecodedImage returns the decoded image.
//
// It is recommended to call DecodeImage and check for errors before using this method.
func (lei *LazyEncodedImage) DecodedImage() image.Image {
	err := lei.DecodeImage()
	if err != nil {
		panic(err)
	}
	return lei.decodedImage
}

// ColorModel returns the Image's color model.
//
// It is recommended to call DecodeConfig and check for errors before using this method.
func (lei *LazyEncodedImage) ColorModel() color.Model {
	err := lei.DecodeConfig()
	if err != nil {
		panic(err)
	}
	return lei.colorModel
}

// Bounds returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
//
// It is recommended to call DecodeConfig and check for errors before using this method.
func (lei *LazyEncodedImage) Bounds() image.Rectangle {
	err := lei.DecodeConfig()
	if err != nil {
		panic(err)
	}
	return *lei.bounds
}

// At returns the color of the pixel at (x, y).
// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
//
// It is recommended to call DecodeImage and check for errors before using this method.
func (lei *LazyEncodedImage) At(x, y int) color.Color {
	err := lei.DecodeImage()
	if err != nil {
		panic(err)
	}
	return lei.decodedImage.At(x, y)
}
