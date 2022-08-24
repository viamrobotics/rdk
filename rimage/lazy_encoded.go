package rimage

import (
	"context"
	"image"
	"image/color"
	"sync"
)

// LazyEncodedImage defers the decoding of an image until necessary.
type LazyEncodedImage struct {
	imgBytes []byte
	mimeType string
	width    int
	height   int

	decodeOnce   sync.Once
	decodeErr    interface{}
	decodedImage image.Image
}

// NewLazyEncodedImage returns a new image that will only get decoded once actual data is needed
// from it. This is helpful for zero copy scenarios.
// NOTE: Usage of an image that would fail to decode causes a lazy panic.
func NewLazyEncodedImage(imgBytes []byte, mimeType string, width, height int) image.Image {
	return &LazyEncodedImage{
		imgBytes: imgBytes,
		mimeType: mimeType,
		width:    width,
		height:   height,
	}
}

func (lei *LazyEncodedImage) decode() {
	lei.decodeOnce.Do(func() {
		defer func() {
			if err := recover(); err != nil {
				lei.decodeErr = err
			}
		}()
		lei.decodedImage, lei.decodeErr = decodeImage(
			context.Background(),
			lei.imgBytes,
			lei.mimeType,
			lei.width,
			lei.height,
			false,
		)
	})
	if lei.decodeErr != nil {
		panic(lei.decodeErr)
	}
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

// ColorModel returns the Image's color model.
func (lei *LazyEncodedImage) ColorModel() color.Model {
	lei.decode()
	return lei.decodedImage.ColorModel()
}

// Bounds returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
func (lei *LazyEncodedImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, lei.width, lei.height)
}

// At returns the color of the pixel at (x, y).
// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
func (lei *LazyEncodedImage) At(x, y int) color.Color {
	lei.decode()
	return lei.decodedImage.At(x, y)
}
