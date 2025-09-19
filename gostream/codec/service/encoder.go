package generic

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
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

// EncoderFactory is a generic service encoder factory.
type EncoderFactory struct {
	encoder generic.Service
	logger  logging.Logger
}

// NewEncoderFactory creates a new generic service encoder factory.
func NewEncoderFactory(encoder generic.Service, logger logging.Logger) *EncoderFactory {
	return &EncoderFactory{encoder: encoder, logger: logger}
}

// New creates a new generic service encoder.
func (f *EncoderFactory) New(width, height, frameRate int, logger logging.Logger) (codec.VideoEncoder, error) {
	return &Encoder{encoder: f.encoder, logger: logger}, nil
}

// MIMEType returns the MIME type of the encoder.
func (f *EncoderFactory) MIMEType() string {
	return "video/H264"
}

// Encoder is a generic service encoder.
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
	imageBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	cmd := map[string]interface{}{"image": imageBase64}

	resp, err := e.encoder.DoCommand(ctx, cmd)
	if err != nil {
		fmt.Println("error from encoder: ", err)
		return nil, errors.Wrap(err, "failed to encode")
	}

	frameBytes, ok := resp["payload"].(string)
	if !ok {
		return nil, errors.New("payload is not a string")
	}
	b, err := base64.StdEncoding.DecodeString(frameBytes)
	if err != nil {
		fmt.Println("error decoding payload base64: ", err)
		return nil, errors.Wrap(err, "failed to decode payload base64")
	}
	return b, nil
}

// Close closes the encoder.
func (e *Encoder) Close() error {
	return nil
}
