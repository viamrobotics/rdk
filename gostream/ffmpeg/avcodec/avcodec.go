//go:build cgo && linux && !android

// Package avcodec is a wrapper around FFmpeg/release6.1.
// See: https://github.com/FFmpeg/FFmpeg/tree/release/6.1
package avcodec

//#cgo CFLAGS: -I${SRCDIR}/../include
//#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../Linux-aarch64/lib -lavformat -lavcodec -lavutil -lm
//#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../Linux-x86_64/lib -lavformat -lavcodec -lavutil -lm
//#cgo linux,arm LDFLAGS: -L${SRCDIR}/../Linux-armv7l/lib -lavformat -lavcodec -lavutil -lm
//#include <libavformat/avformat.h>
//#include <libavcodec/avcodec.h>
//#include <libavcodec/packet.h>
//#include <libavutil/avutil.h>
import "C"

import (
	"unsafe"

	"go.viam.com/rdk/gostream/ffmpeg/avlog"
)

// AvPixFmtYuv420p the pixel format AV_PIX_FMT_YUV420P
const AvPixFmtYuv420p = C.AV_PIX_FMT_YUV420P

type (
	// Codec an AVCodec
	Codec C.struct_AVCodec
	// Context an AVCodecContext
	Context C.struct_AVCodecContext
	// Dictionary an AVDictionary
	Dictionary C.struct_AVDictionary
	// Frame an AVFrame
	Frame C.struct_AVFrame
	// Packet an AVPacket
	Packet C.struct_AVPacket
	// PixelFormat an AVPixelFormat
	PixelFormat C.enum_AVPixelFormat
)

// FindEncoderByName Find a registered encoder with the specified name.
//
// @param name the name of the requested encoder
// @return An encoder if one was found, NULL otherwise.
func FindEncoderByName(c string) *Codec {
	return (*Codec)(C.avcodec_find_encoder_by_name(C.CString(c)))
}

// PacketAlloc Allocate an AVPacket and set its fields to default values.  The resulting
// struct must be freed using av_packet_free().
//
// @return An AVPacket filled with default values or NULL on failure.
//
// @note this only allocates the AVPacket itself, not the data buffers. Those
// must be allocated through other means such as av_new_packet.
//
// @see av_new_packet
func PacketAlloc() *Packet {
	return (*Packet)(C.av_packet_alloc())
}

// AllocContext3 Allocate an AVCodecContext and set its fields to default values. The
// resulting struct should be freed with avcodec_free_context().
//
// @param codec if non-NULL, allocate private data and initialize defaults
//
//	for the given codec. It is illegal to then call avcodec_open2()
//	with a different codec.
//	If NULL, then the codec-specific defaults won't be initialized,
//	which may result in suboptimal default settings (this is
//	important mainly for encoders, e.g. libx264).
//
// @return An AVCodecContext filled with default values or NULL on failure.
func (c *Codec) AllocContext3() *Context {
	return (*Context)(C.avcodec_alloc_context3((*C.struct_AVCodec)(c)))
}

// SetEncodeParams sets the context's width, height, pixel format (pxlFmt), if it has b-frames and GOP size.
func (ctxt *Context) SetEncodeParams(width, height int, pxlFmt PixelFormat, hasBFrames bool, gopSize int) {
	ctxt.width = C.int(width)
	ctxt.height = C.int(height)
	ctxt.bit_rate = 2000000
	ctxt.gop_size = C.int(gopSize)
	if hasBFrames {
		ctxt.has_b_frames = 1
	} else {
		ctxt.has_b_frames = 0
	}
	ctxt.pix_fmt = int32(pxlFmt)
}

// SetFramerate sets the context's framerate
func (ctxt *Context) SetFramerate(fps int) {
	ctxt.framerate.num = C.int(fps)
	ctxt.framerate.den = C.int(1)

	// timebase should be 1/framerate
	ctxt.time_base.num = C.int(1)
	ctxt.time_base.den = C.int(fps)
}

// Open2 Initialize the AVCodecContext to use the given AVCodec. Prior to using this
// function the context has to be allocated with avcodec_alloc_context3().
//
// The functions avcodec_find_decoder_by_name(), avcodec_find_encoder_by_name(),
// avcodec_find_decoder() and avcodec_find_encoder() provide an easy way for
// retrieving a codec.
//
// Depending on the codec, you might need to set options in the codec context
// also for decoding (e.g. width, height, or the pixel or audio sample format in
// the case the information is not available in the bitstream, as when decoding
// raw audio or video).
//
// Options in the codec context can be set either by setting them in the options
// AVDictionary, or by setting the values in the context itself, directly or by
// using the av_opt_set() API before calling this function.
//
// Example:
// @code
// av_dict_set(&opts, "b", "2.5M", 0);
// codec = avcodec_find_decoder(AV_CODEC_ID_H264);
// if (!codec)
//
//	exit(1);
//
// context = avcodec_alloc_context3(codec);
//
// if (avcodec_open2(context, codec, opts) < 0)
//
//	exit(1);
//
// @endcode
//
// In the case AVCodecParameters are available (e.g. when demuxing a stream
// using libavformat, and accessing the AVStream contained in the demuxer), the
// codec parameters can be copied to the codec context using
// avcodec_parameters_to_context(), as in the following example:
//
// @code
// AVStream *stream = ...;
// context = avcodec_alloc_context3(codec);
// if (avcodec_parameters_to_context(context, stream->codecpar) < 0)
//
//	exit(1);
//
// if (avcodec_open2(context, codec, NULL) < 0)
//
//	exit(1);
//
// @endcode
//
// @note Always call this function before using decoding routines (such as
// @ref avcodec_receive_frame()).
//
// @param avctx The context to initialize.
// @param codec The codec to open this context for. If a non-NULL codec has been
//
//	previously passed to avcodec_alloc_context3() or
//	for this context, then this parameter MUST be either NULL or
//	equal to the previously passed codec.
//
// @param options A dictionary filled with AVCodecContext and codec-private
//
//	options, which are set on top of the options already set in
//	avctx, can be NULL. On return this object will be filled with
//	options that were not found in the avctx codec context.
//
// @return zero on success, a negative value on error
// @see avcodec_alloc_context3(), avcodec_find_decoder(), avcodec_find_encoder(),
//
//	av_dict_set(), av_opt_set(), av_opt_find(), avcodec_parameters_to_context()
func (ctxt *Context) Open2(c *Codec, d **Dictionary) int {
	return int(C.avcodec_open2((*C.struct_AVCodecContext)(ctxt), (*C.struct_AVCodec)(c), (**C.struct_AVDictionary)(unsafe.Pointer(d))))
}

// Close a given AVCodecContext and free all the data associated with it
// (but not the AVCodecContext itself).
//
// Calling this function on an AVCodecContext that hasn't been opened will free
// the codec-specific data allocated in avcodec_alloc_context3() with a non-NULL
// codec. Subsequent calls will do nothing.
//
// @note Do not use this function. Use avcodec_free_context() to destroy a
// codec context (either open or closed). Opening and closing a codec context
// multiple times is not supported anymore -- use multiple codec contexts
// instead.
func (ctxt *Context) Close() int {
	return int(C.avcodec_close((*C.struct_AVCodecContext)(ctxt)))
}

// Unref Wipe the packet.
//
// Unreference the buffer referenced by the packet and reset the
// remaining packet fields to their default values.
//
// @param pkt The packet to be unreferenced.
func (p *Packet) Unref() {
	C.av_packet_unref((*C.struct_AVPacket)(p))
}

// SendFrame Supply a raw video or audio frame to the encoder. Use avcodec_receive_packet()
// to retrieve buffered output packets.
//
// @param avctx     codec context
// @param[in] frame AVFrame containing the raw audio or video frame to be encoded.
//
//	Ownership of the frame remains with the caller, and the
//	encoder will not write to the frame. The encoder may create
//	a reference to the frame data (or copy it if the frame is
//	not reference-counted).
//	It can be NULL, in which case it is considered a flush
//	packet.  This signals the end of the stream. If the encoder
//	still has packets buffered, it will return them after this
//	call. Once flushing mode has been entered, additional flush
//	packets are ignored, and sending frames will return
//	AVERROR_EOF.
//
//	For audio:
//	If AV_CODEC_CAP_VARIABLE_FRAME_SIZE is set, then each frame
//	can have any number of samples.
//	If it is not set, frame->nb_samples must be equal to
//	avctx->frame_size for all frames except the last.
//	The final frame may be smaller than avctx->frame_size.
//
// @retval 0                 success
// @retval AVERROR(EAGAIN)   input is not accepted in the current state - user must
//
//	read output with avcodec_receive_packet() (once all
//	output is read, the packet should be resent, and the
//	call will not fail with EAGAIN).
//
// @retval AVERROR_EOF       the encoder has been flushed, and no new frames can
//
//	be sent to it
//
// @retval AVERROR(EINVAL)   codec not opened, it is a decoder, or requires flush
// @retval AVERROR(ENOMEM)   failed to add packet to internal queue, or similar
// @retval "another negative error code" legitimate encoding errors
func (ctxt *Context) SendFrame(f *Frame) int {
	return int(C.avcodec_send_frame((*C.struct_AVCodecContext)(ctxt), (*C.struct_AVFrame)(f)))
}

// ReceivePacket Read encoded data from the encoder.
//
// @param avctx codec context
// @param avpkt This will be set to a reference-counted packet allocated by the
//
//	encoder. Note that the function will always call
//	av_packet_unref(avpkt) before doing anything else.
//
// @retval 0               success
// @retval AVERROR(EAGAIN) output is not available in the current state - user must
//
//	try to send input
//
// @retval AVERROR_EOF     the encoder has been fully flushed, and there will be no
//
//	more output packets
//
// @retval AVERROR(EINVAL) codec not opened, or it is a decoder
// @retval "another negative error code" legitimate encoding errors
func (ctxt *Context) ReceivePacket(a *Packet) int {
	return int(C.avcodec_receive_packet((*C.struct_AVCodecContext)(ctxt), (*C.struct_AVPacket)(a)))
}

// Data returns the packet's data
func (p *Packet) Data() *uint8 {
	return (*uint8)(p.data)
}

// Size returns the packet size
func (p *Packet) Size() int {
	return int(p.size)
}

// EncoderIsAvailable returns true if the given encoder is available, false otherwise.
func EncoderIsAvailable(enc string) bool {
	// Quiet logging during function execution, but reset afterward.
	lvl := avlog.GetLevel()
	defer avlog.SetLevel(lvl)
	avlog.SetLevel(avlog.LogQuiet)

	codec := FindEncoderByName(enc)
	if codec == nil {
		return false
	}

	context := codec.AllocContext3()
	if context == nil {
		return false
	}

	// Only need positive values
	context.SetEncodeParams(1, 1, AvPixFmtYuv420p, false, 1)
	context.SetFramerate(1)

	return context.Open2(codec, nil) == 0
}
