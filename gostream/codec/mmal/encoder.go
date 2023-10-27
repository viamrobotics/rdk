//go:build mmal

// Package mmal contains the mmal video codec.
package mmal

import (
	"context"
	"image"

	ourcodec "go.viam.com/rdk/gostream/codec"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/codec/mmal"
	"github.com/pion/mediadevices/pkg/prop"
)

type encoder struct {
	codec  codec.ReadCloser
	img    image.Image
	logger golog.Logger
}

// Gives suitable results. Probably want to make this configurable this in the future.
const bitrate = 3_200_000

// NewEncoder returns an MMAL encoder that can encode images of the given width and height. It will
// also ensure that it produces key frames at the given interval.
func NewEncoder(width, height, keyFrameInterval int, logger golog.Logger) (ourcodec.VideoEncoder, error) {
	enc := &encoder{logger: logger}

	var builder codec.VideoEncoderBuilder
	params, err := mmal.NewParams()
	if err != nil {
		return nil, err
	}
	builder = &params
	params.BitRate = bitrate
	params.KeyFrameInterval = keyFrameInterval

	codec, err := builder.BuildVideoEncoder(enc, prop.Media{
		Video: prop.Video{
			Width:  width,
			Height: height,
		},
	})
	if err != nil {
		return nil, err
	}
	enc.codec = codec

	return enc, nil
}

// Read returns an image for codec to process.
func (v *encoder) Read() (img image.Image, release func(), err error) {
	return v.img, nil, nil
}

// Encode asks the codec to process the given image.
func (v *encoder) Encode(_ context.Context, img image.Image) ([]byte, error) {
	v.img = img
	data, release, err := v.codec.Read()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	release()
	return dataCopy, err
}
