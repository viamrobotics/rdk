package rimage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"sync"
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
		// Callsites are encouraged to check for a mime type and explicitly call
		// `DetectContentType`/log unexpected cases if desired.
		mimeType = http.DetectContentType(imgBytes)
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
	if err := lei.DecodeConfig(); err != nil {
		return err
	}
	if err := lei.DecodeImage(); err != nil {
		return err
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

// ColorModelSafe returns the Image's color model.
//
// This method is a safer alternative to the ColorModel method, as it provides error handling
// instead of panicking, ensuring that the caller can manage any decoding issues gracefully.
func (lei *LazyEncodedImage) ColorModelSafe() (color.Model, error) {
	err := lei.DecodeConfig()
	if err != nil {
		return nil, err
	}
	return lei.colorModel, nil
}

// ColorModel returns the Image's color model.
//
// It is recommended to call DecodeConfig and check for errors before using this method.
func (lei *LazyEncodedImage) ColorModel() color.Model {
	model, err := lei.ColorModelSafe()
	if err != nil {
		panic(err)
	}
	return model
}

// BoundsSafe returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
//
// This method is a safer alternative to the Bounds method, as it provides error handling
// instead of panicking, allowing the caller to handle any issues that arise during decoding.
func (lei *LazyEncodedImage) BoundsSafe() (image.Rectangle, error) {
	err := lei.DecodeConfig()
	if err != nil {
		return image.Rectangle{}, err
	}
	return *lei.bounds, nil
}

// Bounds returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
//
// It is recommended to call DecodeConfig and check for errors before using this method, or use BoundsSafe.
// This method is considered unsafe as it will panic if the image is not decoded.
func (lei *LazyEncodedImage) Bounds() image.Rectangle {
	bounds, err := lei.BoundsSafe()
	if err != nil {
		panic(err)
	}
	return bounds
}

// AtSafe returns the color of the pixel at (x, y).
//
// This method is a safer alternative to the At method, as it provides error handling
// instead of panicking, enabling the caller to manage any decoding errors appropriately.
func (lei *LazyEncodedImage) AtSafe(x, y int) (color.Color, error) {
	err := lei.DecodeImage()
	if err != nil {
		return nil, err
	}
	return lei.decodedImage.At(x, y), nil
}

// At returns the color of the pixel at (x, y).
//
// It is recommended to call DecodeImage and check for errors before using this method, or use AtSafe.
// This method is unsafe as it will panic if the image is not decoded.
func (lei *LazyEncodedImage) At(x, y int) color.Color {
	c, err := lei.AtSafe(x, y)
	if err != nil {
		panic(err)
	}
	return c
}
