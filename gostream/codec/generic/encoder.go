package generic

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"

	"github.com/pkg/errors"
	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/generic"
)

var (
	//ErrNotAYUV is return when the image is not a YUV
	ErrNotAYUV = errors.New("image is not a YUV")
	//ErrUnsupportedImageType is returned when the image type is not supported
	ErrUnsupportedImageType = errors.New("unsupported image type")
)

// EncoderFactory is a generic encoder factory.
type EncoderFactory struct {
	encoder generic.Service
	logger  logging.Logger
}

// NewEncoderFactory creates a new generic encoder factory.
func NewEncoderFactory(encoder generic.Service, logger logging.Logger) *EncoderFactory {
	return &EncoderFactory{encoder: encoder, logger: logger}
}

// New creates a new generic encoder.
func (f *EncoderFactory) New(width, height, frameRate int, logger logging.Logger) (codec.VideoEncoder, error) {
	return &Encoder{encoder: f.encoder, logger: logger}, nil
}

// MIMEType returns the MIME type of the encoder.
func (f *EncoderFactory) MIMEType() string {
	return "video/H264"
}

// Encoder is a generic encoder.
type Encoder struct {
	encoder generic.Service
	logger  logging.Logger
}

// Encode encodes an image into a byte stream.
func (e *Encoder) Encode(ctx context.Context, img image.Image) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, nil); err != nil {
		return nil, errors.Wrap(err, "failed to encode image to jpeg")
	}

	cmd := map[string]interface{}{"image": buf.Bytes()}

	resp, err := e.encoder.DoCommand(ctx, cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode")
	}

	encoded, ok := resp["payload"].([]byte)
	if !ok {
		return nil, errors.New("payload not found in response")
	}
	return encoded, nil
}

// Close closes the encoder.
func (e *Encoder) Close() error {
	return nil
}
