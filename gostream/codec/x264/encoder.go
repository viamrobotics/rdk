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

// Macroblock alignment workaround.
//
// H.264 encodes in 16x16 macroblocks. Pion's x264 wrapper (bridge.h
// enc_encode) swaps pic_in.img.plane[] to point at Go pixel data on every
// frame but never updates pic_in.img.i_stride[]. When the input width or
// height isn't a multiple of 16, x264's stride assumption diverges from
// Go's image.YCbCr layout, so x264 reads rows at the wrong memory offset.
// Symptom is horizontal stripes when width is off, unassemblable frames
// when height is off.
//
// Workaround: crop input to the nearest 16-multiple before handing to
// pion so the stride mismatch can't happen. Aligned inputs pass through
// unchanged.
//
// Remove this entire mechanism (the macroblockAlign constant, the
// needsCrop/dstBounds/scratchRGBA fields, the alignment logic in
// NewEncoder, and the crop in Encode) once
// https://github.com/pion/mediadevices/issues/707 ships upstream.
const macroblockAlign = 16

type encoder struct {
	codec  codec.ReadCloser
	img    image.Image
	logger logging.Logger

	// Fields below implement the macroblock alignment workaround; delete
	// with the rest of the workaround.
	needsCrop   bool
	dstBounds   image.Rectangle
	scratchRGBA *image.RGBA
}

// NewEncoder returns an x264 encoder that can encode images of the given width and height. It will
// also ensure that it produces key frames at the given interval.
func NewEncoder(width, height, keyFrameInterval int, logger logging.Logger) (ourcodec.VideoEncoder, error) {
	// Check to make sure dimensions are even.
	if width%2 != 0 || height%2 != 0 {
		return nil, errors.New("x264 encoder does not support odd dimensions. " +
			"Please provide frames with even dimensions for width and height")
	}

	// Round dims DOWN to the nearest 16-multiple. If input is already
	// aligned, alignedW == width and needsCrop stays false (no-op path).
	alignedW := width &^ (macroblockAlign - 1)
	alignedH := height &^ (macroblockAlign - 1)
	enc := &encoder{
		logger:    logger,
		needsCrop: alignedW != width || alignedH != height,
	}
	if enc.needsCrop {
		// Pre-allocate one scratch buffer for the whole encoder lifetime;
		// reused per frame in Encode() to avoid churning allocations.
		enc.dstBounds = image.Rect(0, 0, alignedW, alignedH)
		enc.scratchRGBA = image.NewRGBA(enc.dstBounds)
		logger.Infow("x264: input dims not macroblock-aligned; cropping per frame",
			"from", []int{width, height},
			"to", []int{alignedW, alignedH})
	}

	var builder codec.VideoEncoderBuilder
	params, err := x264.NewParams()
	if err != nil {
		return nil, err
	}
	builder = &params
	params.KeyFrameInterval = keyFrameInterval
	params.BitRate = calcBitrateFromResolution(alignedW, alignedH, float32(params.KeyFrameInterval))
	params.LogLevel = x264.LogWarning

	codec, err := builder.BuildVideoEncoder(enc, prop.Media{
		Video: prop.Video{
			Width:  alignedW,
			Height: alignedH,
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
		// Copy pixels into the aligned scratch buffer. draw.Draw handles
		// stride correctly across YCbCr / RGBA / Gray inputs.
		src := img.Bounds()
		draw.Draw(v.scratchRGBA, v.dstBounds, img, src.Min, draw.Src)
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
