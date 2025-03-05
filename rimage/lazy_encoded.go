package rimage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"strings"
	"sync"

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

// Helper method for checking generic typed errors such as the one in lei structs.
// TODO(hexbabe): why?
func checkError(err interface{}) error {
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

// safeCall executes the given function and catches any panics, converting them to errors.
// It returns any error that occurred during execution.
func safeCall(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	f()
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
	return checkError(lei.decodeImageErr)
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
	return checkError(lei.decodeConfigErr)
}

// DecodeAll decodes the image and its configuration. Returns nil if no errors occurred.
// This method is idempotent.
func (lei *LazyEncodedImage) DecodeAll() error {
	configErr := lei.DecodeConfig()
	decodeErr := lei.DecodeImage()
	if configErr != nil || decodeErr != nil {
		return errors.Join(configErr, decodeErr)
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
func (lei *LazyEncodedImage) DecodedImage() (image.Image, error) {
	err := lei.DecodeImage()
	if err != nil {
		return nil, err
	}
	return lei.decodedImage, nil
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

// ColorModelSafe returns the Image's color model.
//
// This method is a safer alternative to the ColorModel method, as it provides error handling
// instead of panicking, ensuring that the caller can manage any decoding issues gracefully.
func (lei *LazyEncodedImage) ColorModelSafe() (color.Model, error) {
	var model color.Model
	err := safeCall(func() {
		model = lei.ColorModel()
	})
	return model, err
}

// Bounds returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
//
// It is recommended to call DecodeConfig and check for errors before using this method.
// This method is considered unsafe as it will panic if the image is not decoded.
func (lei *LazyEncodedImage) Bounds() image.Rectangle {
	err := lei.DecodeConfig()
	if err != nil {
		panic(err)
	}
	return *lei.bounds
}

// BoundsSafe returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
//
// This method is a safer alternative to the Bounds method, as it provides error handling
// instead of panicking, allowing the caller to handle any issues that arise during decoding.
func (lei *LazyEncodedImage) BoundsSafe() (image.Rectangle, error) {
	var bounds image.Rectangle
	err := safeCall(func() {
		bounds = lei.Bounds()
	})
	return bounds, err
}

// At returns the color of the pixel at (x, y).
// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
//
// It is recommended to call DecodeImage and check for errors before using this method.
// This method is unsafe as it will panic if the image is not decoded.
func (lei *LazyEncodedImage) At(x, y int) color.Color {
	err := lei.DecodeImage()
	if err != nil {
		panic(err)
	}
	return lei.decodedImage.At(x, y)
}

// AtSafe returns the color of the pixel at (x, y).
// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
//
// This method is a safer alternative to the At method, as it provides error handling
// instead of panicking, enabling the caller to manage any decoding errors appropriately.
func (lei *LazyEncodedImage) AtSafe(x, y int) (color.Color, error) {
	var c color.Color
	err := safeCall(func() {
		c = lei.At(x, y)
	})
	return c, err
}
