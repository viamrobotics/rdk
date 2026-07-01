package camera

import (
	"braces.dev/errtrace"
	"github.com/pkg/errors"
)

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
	return errtrace.Wrap(errors.Errorf("image type %q not supported", s))
}
