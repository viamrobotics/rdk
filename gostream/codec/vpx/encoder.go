// Package vpx contains the vpx video codec.
package vpx

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	"github.com/pion/mediadevices/pkg/prop"

	ourcodec "go.viam.com/rdk/gostream/codec"
)

type encoder struct {
	codec  codec.ReadCloser
	img    image.Image
	logger golog.Logger
}

// Version determines the version of a vpx codec.
type Version string

// The set of allowed vpx versions.
const (
	Version8 Version = "vp8"
	Version9 Version = "vp9"
)

// Gives suitable results. Probably want to make this configurable this in the future.
const bitrate = 3_200_000

// NewEncoder returns a vpx encoder of the given type that can encode images of the given width and height. It will
// also ensure that it produces key frames at the given interval.
func NewEncoder(codecVersion Version, width, height, keyFrameInterval int, logger golog.Logger) (ourcodec.VideoEncoder, error) {
	enc := &encoder{logger: logger}

	var builder codec.VideoEncoderBuilder
	switch codecVersion {
	case Version8:
		params, err := vpx.NewVP8Params()
		if err != nil {
			return nil, err
		}
		builder = &params
		params.BitRate = bitrate
		params.KeyFrameInterval = keyFrameInterval
	case Version9:
		params, err := vpx.NewVP9Params()
		if err != nil {
			return nil, err
		}
		builder = &params
		params.BitRate = bitrate
		params.KeyFrameInterval = keyFrameInterval
	default:
		return nil, fmt.Errorf("unsupported vpx version: %s", codecVersion)
	}

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
