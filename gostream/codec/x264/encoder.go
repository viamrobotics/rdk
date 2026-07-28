// Package x264 contains the x264 video codec.
package x264

import (
	"context"
	"errors"
	"image"
	"image/draw"

	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/codec/x264"
	"github.com/pion/mediadevices/pkg/prop"

	ourcodec "go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
)

// Workaround for pion/mediadevices#707: pion's x264 wrapper doesn't update
// pic_in.img.i_stride[] when the input width isn't macroblock-aligned,
// causing corrupt frames. Crop input width to the nearest 16-multiple
// before handing to pion. Remove once pion/mediadevices#707 ships upstream.
const macroblockAlign = 16

type encoder struct {
	codec       codec.ReadCloser
	img         image.Image
	logger      logging.Logger
	needsCrop   bool
	dstBounds   image.Rectangle
	scratchRGBA *image.RGBA
}

// NewEncoder returns an x264 encoder that can encode images of the given width and height. It will
// also ensure that it produces key frames at the given interval.
func NewEncoder(width, height, keyFrameInterval int, logger logging.Logger) (ourcodec.VideoEncoder, error) {
	if width%2 != 0 || height%2 != 0 {
		return nil, errors.New("x264 encoder does not support odd dimensions. " +
			"Please provide frames with even dimensions for width and height")
	}

	alignedW := width &^ (macroblockAlign - 1)
	enc := &encoder{
		logger:    logger,
		needsCrop: alignedW != width,
	}
	if enc.needsCrop {
		enc.dstBounds = image.Rect(0, 0, alignedW, height)
		enc.scratchRGBA = image.NewRGBA(enc.dstBounds)
		logger.Infow("x264: input width not macroblock-aligned; cropping per frame",
			"from_width", width, "to_width", alignedW, "height", height)
	}

	var builder codec.VideoEncoderBuilder
	params, err := x264.NewParams()
	if err != nil {
		return nil, err
	}
	builder = &params
	params.KeyFrameInterval = keyFrameInterval
	params.BitRate = calcBitrateFromResolution(alignedW, height, float32(params.KeyFrameInterval))
	params.LogLevel = x264.LogWarning

	codec, err := builder.BuildVideoEncoder(enc, prop.Media{
		Video: prop.Video{
			Width:  alignedW,
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
	if v.needsCrop {
		draw.Draw(v.scratchRGBA, v.dstBounds, img, img.Bounds().Min, draw.Src)
		img = v.scratchRGBA
	}
	v.img = img
	data, release, err := v.codec.Read()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	release()
	return dataCopy, err
}

// Close closes the encoder.
func (v *encoder) Close() error {
	return v.codec.Close()
}
