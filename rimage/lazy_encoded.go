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

	decodeOnce   sync.Once
	decodeErr    interface{}
	decodedImage image.Image

	decodeConfigOnce sync.Once
	decodeConfigErr  interface{}
	bounds           *image.Rectangle
	colorModel       color.Model
}

// NewLazyEncodedImage returns a new image that will only get decoded once actual data is needed
// from it. This is helpful for zero copy scenarios. If a width or height of the image is unknown,
// pass 0 or -1; when done a decode will happen on Bounds. In the future this can probably go
// away with reading all metadata from the header of the image bytes.
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

// decode performs the actual decoding of the image data.
// If there's an error, it stores it.
func (lei *LazyEncodedImage) decode() {
	lei.decodeOnce.Do(func() {
		defer func() {
			if err := recover(); err != nil {
				lei.decodeErr = err
			}
		}()
		lei.decodedImage, lei.decodeErr = DecodeImage(
			context.Background(),
			lei.imgBytes,
			lei.mimeType,
		)
	})
}

// decodeConfig decodes just the image configuration.
// If there's an error, it stores it.
func (lei *LazyEncodedImage) decodeConfig() {
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

// GetErrors returns any errors that occurred during decoding the image or its configuration.
// Returns nil if no errors occurred.
func (lei *LazyEncodedImage) GetErrors() error {
	// Helper function for checking lei errors
	checkError := func(err interface{}, errMsg string) error {
		if err != nil {
			switch e := err.(type) {
			case error:
				return e
			default:
				return fmt.Errorf("%s: %v", errMsg, err)
			}
		}
		return nil
	}

	lei.decodeConfig()
	err := checkError(lei.decodeConfigErr, "decode lazy encoded image config error")

	lei.decode()
	err = multierr.Combine(err, checkError(lei.decodeErr, "decode lazy encoded image error"))

	return fmt.Errorf("lazy encoded image error(s): %w", err)
}

// DecodedImage returns the decoded image or nil if decoding failed.
func (lei *LazyEncodedImage) DecodedImage() image.Image {
	lei.decode()
	if lei.decodeErr != nil {
		logging.Global().Errorf("Failed to decode image (DecodedImage): %v", lei.decodeErr)
		return nil
	}
	return lei.decodedImage
}

// ColorModel returns the Image's color model.
// Returns a default color model if decoding failed.
func (lei *LazyEncodedImage) ColorModel() color.Model {
	lei.decodeConfig()
	if lei.decodeConfigErr != nil {
		logging.Global().Errorf("Failed to decode (color model): %v", lei.decodeConfigErr)
		return color.RGBAModel
	}
	return lei.colorModel
}

// Bounds returns the domain for which At can return non-zero color.
// The bounds do not necessarily contain the point (0, 0).
// Returns an empty rectangle if decoding failed.
func (lei *LazyEncodedImage) Bounds() image.Rectangle {
	lei.decodeConfig()
	if lei.decodeConfigErr != nil {
		logging.Global().Errorf("Failed to decode (image bounds): %v", lei.decodeConfigErr)
		return image.Rectangle{}
	}
	if lei.bounds == nil {
		logging.Global().Error("Bounds were nil after decoding configuration.")
		return image.Rectangle{}
	}
	return *lei.bounds
}

// At returns the color of the pixel at (x, y).
// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
// Returns transparent black if decoding failed.
func (lei *LazyEncodedImage) At(x, y int) color.Color {
	lei.decode()
	if lei.decodeErr != nil {
		logging.Global().Errorf("Failed to decode image (At): %v", lei.decodeErr)
		return color.RGBA{}
	}
	if lei.decodedImage == nil {
		logging.Global().Error("Decoded image was nil after decoding.")
		return color.RGBA{}
	}
	return lei.decodedImage.At(x, y)
}
