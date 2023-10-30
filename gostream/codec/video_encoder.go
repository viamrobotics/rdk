// Package codec defines the encoder and factory interfaces for encoding video frames and audio chunks.
package codec

import (
	"context"
	"image"

	"github.com/edaniels/golog"
)

// DefaultKeyFrameInterval is the default interval chosen
// in order to produce high enough quality results at a low
// latency.
const DefaultKeyFrameInterval = 30

// A VideoEncoder is anything that can encode images into bytes. This means that
// the encoder must follow some type of format dictated by a type (see EncoderFactory.MimeType).
// An encoder that produces bytes of different encoding formats per call is invalid.
type VideoEncoder interface {
	Encode(ctx context.Context, img image.Image) ([]byte, error)
}

// A VideoEncoderFactory produces VideoEncoders and provides information about the underlying encoder itself.
type VideoEncoderFactory interface {
	New(height, width, keyFrameInterval int, logger golog.Logger) (VideoEncoder, error)
	MIMEType() string
}
