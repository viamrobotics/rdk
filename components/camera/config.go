package camera

import "fmt"

// ImageType specifies what kind of image stream is coming from the camera.
type ImageType string

// The allowed types of streams that can come from a VideoSource.
const (
	UnspecifiedStream = ImageType("")
	ColorStream       = ImageType("color")
	DepthStream       = ImageType("depth")
)

// NewUnsupportedImageTypeError is when the stream type is unknown.
func NewUnsupportedImageTypeError(s ImageType) error {
	return fmt.Errorf("image type %q not supported", s)
}
