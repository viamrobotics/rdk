package camera

import (
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

// NewUnsupportedStreamError is when the stream type is unknown.
func NewUnsupportedStreamError(s ImageType) error {
	return errors.Errorf("stream of type %q not supported", s)
}
