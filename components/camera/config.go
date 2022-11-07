package camera

import (
	"github.com/pkg/errors"
)

// StreamType specifies what kind of image stream is coming from the camera.
type StreamType string

// The allowed types of streams that can come from a VideoSource.
const (
	UnspecifiedStream = StreamType("")
	ColorStream       = StreamType("color")
	DepthStream       = StreamType("depth")
)

// NewUnsupportedStreamError is when the stream type is unknown.
func NewUnsupportedStreamError(s StreamType) error {
	return errors.Errorf("stream of type %q not supported", s)
}
