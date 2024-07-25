//go:build cgo && linux && !android

// Package h264 uses a V4L2-compatible h.264 hardware encoder (h264_v4l2m2m) to encode images.
package h264

import "C"

import (
	"context"
	"image"
	"time"
	"unsafe"

	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pkg/errors"

	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/gostream/ffmpeg/avcodec"
	"go.viam.com/rdk/gostream/ffmpeg/avutil"
	"go.viam.com/rdk/logging"
)

const (
	// pixelFormat This format is one of the output formats support by the bcm2835-codec at /dev/video11
	// It is also known as YU12. See https://www.kernel.org/doc/html/v4.10/media/uapi/v4l/pixfmt-yuv420.html
	pixelFormat = avcodec.AvPixFmtYuv420p
	// V4l2m2m Is a V4L2 memory-to-memory H.264 hardware encoder.
	V4l2m2m = "h264_v4l2m2m"
	// macroBlock is the encoder boundary block size in bytes.
	macroBlock = 64
	// warmupTime is the time to wait for the encoder to warm up in milliseconds.
	warmupTime = 1000 // 1 second
)

type encoder struct {
	img     image.Image
	reader  video.Reader
	codec   *avcodec.Codec
	context *avcodec.Context
	width   int
	height  int
	frame   *avutil.Frame
	pts     int64
	logger  logging.Logger
}

func (h *encoder) Read() (img image.Image, release func(), err error) {
	return h.img, nil, nil
}

// NewEncoder returns an h264 encoder that can encode images of the given width and height. It will
// also ensure that it produces key frames at the given interval.
func NewEncoder(width, height, keyFrameInterval int, logger logging.Logger) (codec.VideoEncoder, error) {
	h := &encoder{width: width, height: height, logger: logger}

	if h.codec = avcodec.FindEncoderByName(V4l2m2m); h.codec == nil {
		return nil, errors.Errorf("cannot find encoder '%s'", V4l2m2m)
	}

	if h.context = h.codec.AllocContext3(); h.context == nil {
		return nil, errors.New("cannot allocate video codec context")
	}

	h.context.SetEncodeParams(width, height, avcodec.PixelFormat(pixelFormat), false, keyFrameInterval)
	h.context.SetFramerate(keyFrameInterval)

	h.reader = video.ToI420((video.ReaderFunc)(h.Read))

	if h.context.Open2(h.codec, nil) < 0 {
		return nil, errors.New("cannot open codec")
	}

	if h.frame = avutil.FrameAlloc(); h.frame == nil {
		if err := h.Close(); err != nil {
			return nil, errors.Wrap(err, "cannot close codec")
		}
		return nil, errors.New("cannot alloc frame")
	}

	// give the encoder some time to warm up
	time.Sleep(warmupTime * time.Millisecond)

	return h, nil
}

func (h *encoder) Encode(ctx context.Context, img image.Image) ([]byte, error) {
	if err := avutil.SetFrame(h.frame, h.width, h.height, pixelFormat); err != nil {
		return nil, errors.Wrap(err, "cannot set frame properties")
	}

	if ret := avutil.FrameMakeWritable(h.frame); ret < 0 {
		return nil, errors.Wrap(avutil.ErrorFromCode(ret), "cannot make frame writable")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	h.img = img
	yuvImg, release, err := h.reader.Read()
	defer release()

	if err != nil {
		return nil, errors.Wrap(err, "cannot read image")
	}

	h.frame.SetFrameFromImgMacroAlign(yuvImg.(*image.YCbCr), macroBlock)
	h.frame.SetFramePTS(h.pts)
	h.pts++

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return h.encodeBytes(ctx)
	}
}

func (h *encoder) encodeBytes(ctx context.Context) ([]byte, error) {
	pkt := avcodec.PacketAlloc()
	if pkt == nil {
		return nil, errors.New("cannot allocate packet")
	}
	defer pkt.Unref()
	defer avutil.FrameUnref(h.frame)

	if ret := h.context.SendFrame((*avcodec.Frame)(unsafe.Pointer(h.frame))); ret < 0 {
		return nil, errors.Wrap(avutil.ErrorFromCode(ret), "cannot supply raw video to encoder")
	}

	var bytes []byte
	var ret int
loop:
	// See "send/receive encoding and decoding API overview" from https://ffmpeg.org/doxygen/3.4/group__lavc__encdec.html.
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		ret = h.context.ReceivePacket(pkt)
		switch ret {
		case avutil.Success:
			payload := C.GoBytes(unsafe.Pointer(pkt.Data()), C.int(pkt.Size()))
			bytes = append(bytes, payload...)
			pkt.Unref()
		case avutil.EAGAIN, avutil.EOF:
			break loop
		default:
			return nil, avutil.ErrorFromCode(ret)
		}
	}

	return bytes, nil
}

// Close closes the encoder. It is safe to call this method multiple times.
// It is also safe to call this method after a call to Encode.
func (h *encoder) Close() error {
	if h.frame != nil {
		avutil.FrameUnref(h.frame)
		h.frame = nil
	}
	if h.context != nil {
		h.context.FreeContext()
		h.context = nil
	}

	return nil
}
