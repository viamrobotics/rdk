//go:build cgo && !android

package ffmpeg

//#include <libavformat/avformat.h>
//#include <libavcodec/avcodec.h>
//#include <libavcodec/packet.h>
//#include <libavutil/avutil.h>
//#include <libavutil/imgutils.h>
//#include <libswscale/swscale.h>
// static const char *error2string(int code) { return av_err2str(code); }
import "C"

import (
	"context"
	"image"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	// YUV420P the pixel format AV_PIX_FMT_YUV420P
	YUV420P = C.AV_PIX_FMT_YUV420P
	// Target bitrate in bits per second.
	bitrate = 1_200_000
	// Target bitrate tolerance factor.
	// E.g., 2.0 gives 100% deviation from the target bitrate.
	bitrateDeviation = 2
)

// From https://ffmpeg.org/doxygen/trunk/pixfmt_8h_source.html#l00064
const (
	// PixFmtNone an AV_PIX_FMT_NONE
	PixFmtNone = iota - 1
	// PixFmtYUV420P an AV_PIX_FMT_YUV420P
	PixFmtYUV420P
	// PixFmtYUYV422 an AV_PIX_FMT_YUYV422
	PixFmtYUYV422
	// PixFmtRGB24 an AV_PIX_FMT_RGB24
	PixFmtRGB24
	// PixFmtBGR24 an AV_PIX_FMT_BGR24
	PixFmtBGR24
	// PixFmtYUV422P an AV_PIX_FMT_YUV422P
	PixFmtYUV422P
	// PixFmtYUV444P an AV_PIX_FMT_YUV444P
	PixFmtYUV444P
	// PixFmtYUV410P an AV_PIX_FMT_YUV410P
	PixFmtYUV410P
	// PixFmtYUV411P an AV_PIX_FMT_YUV411P
	PixFmtYUV411P
	// PixFmtGray8 an AV_PIX_FMT_GRAY8
	PixFmtGray8
	// PixFmtMonoWhite an AV_PIX_FMT_MONOWHITE
	PixFmtMonoWhite
	// PixFmtMonoBlack an AV_PIX_FMT_MONOBLACK
	PixFmtMonoBlack
	// PixFmtPal8 an AV_PIX_FMT_PAL8
	PixFmtPal8
	// PixFmtYUVJ420P an AV_PIX_FMT_YUVJ420P
	PixFmtYUVJ420P
	// PixFmtYUVJ422P an AV_PIX_FMT_YUVJ422P
	PixFmtYUVJ422P
	// PixFmtYUVJ444P an AV_PIX_FMT_YUVJ444P
	PixFmtYUVJ444P
	// PixFmtUYVY422 an AV_PIX_FMT_UYVY422
	PixFmtUYVY422
	// PixFmtUYYVYY411 an AV_PIX_FMT_UYYVYY411
	PixFmtUYYVYY411
	// PixFmtBGR8 an AV_PIX_FMT_BGR8
	PixFmtBGR8
)

type (
	// Codec an AVCodec
	Codec struct {
		p  *C.struct_AVCodec
		pp **C.struct_AVCodec
	}
	// CodecContext an AVCodecContext
	CodecContext struct {
		p  *C.struct_AVCodecContext
		pp **C.struct_AVCodecContext
	}

	// Packet an AVPacket
	Packet struct {
		p  *C.struct_AVPacket
		pp **C.struct_AVPacket
	}
	// PixelFormat an AVPixelFormat
	PixelFormat C.enum_AVPixelFormat
	// CodecID an AVCodecID
	CodecID C.enum_AVCodecID
)

// From https://ffmpeg.org/doxygen/4.0/avcodec_8h_source.html#l00215
const (
	CodecIDNone = iota
	CodecIDMPEG1VIDEO
	CodecIDMPEG2VIDEO
	CodecIDH261
	CodecIDH263
	CodecIDRV10
	CodecIDRV20
	CodecIDMJPEG
	CodecIDMJPEGB
	CodecIDLJPEG
	CodecIDSP5X
	CodecIDJPEGLS
	CodecIDMPEG4
	CodecIDRAWVIDEO
	CodecIDMSMPEG4V1
	CodecIDMSMPEG4V2
	CodecIDMSMPEG4V3
	CodecIDWMV1
	CodecIDWMV2
	CodecIDH263P
	CodecIDH263I
	CodecIDFLV1
	CodecIDSVQ1
	CodecIDSVQ3
	CodecIDDVVIDEO
	CodecIDHUFFYUV
	CodecIDCYUV
	CodecIDH264
	CodecIDINDEO3
	CodecIDVP3
)

// FindEncoderByName Find a registered encoder with the specified name.
//
// @param name the name of the requested encoder
// @return An encoder if one was found, NULL otherwise.
func FindEncoderByName(c string) (Codec, error) {
	codec := C.avcodec_find_encoder_by_name(C.CString(c))
	if codec == nil {
		return Codec{}, errors.Errorf("cannot find encoder %s", c)
	}
	return Codec{
		p:  codec,
		pp: &codec,
	}, nil
}

// ToString get the string representation of the codec ID
func (c *CodecID) ToString() string {
	switch *c {
	case CodecIDNone:
		return "None"
	case CodecIDMPEG1VIDEO:
		return "MPEG1VIDEO"
	case CodecIDMPEG2VIDEO:
		return "MPEG2VIDEO"
	case CodecIDH261:
		return "H261"
	case CodecIDH263:
		return "H263"
	case CodecIDRV10:
		return "RV10"
	case CodecIDRV20:
		return "RV20"
	case CodecIDMJPEG:
		return "MJPEG"
	case CodecIDMJPEGB:
		return "MJPEGB"
	case CodecIDLJPEG:
		return "LJPEG"
	case CodecIDSP5X:
		return "SP5X"
	case CodecIDJPEGLS:
		return "JPEGLS"
	case CodecIDMPEG4:
		return "MPEG4"
	case CodecIDRAWVIDEO:
		return "RAWVIDEO"
	case CodecIDMSMPEG4V1:
		return "MSMPEG4V1"
	case CodecIDMSMPEG4V2:
		return "MSMPEG4V2"
	case CodecIDMSMPEG4V3:
		return "MSMPEG4V3"
	case CodecIDWMV1:
		return "WMV1"
	case CodecIDWMV2:
		return "WMV2"
	case CodecIDH263P:
		return "H263P"
	case CodecIDH263I:
		return "H263I"
	case CodecIDFLV1:
		return "FLV1"
	case CodecIDSVQ1:
		return "SVQ1"
	case CodecIDSVQ3:
		return "SVQ3"
	case CodecIDDVVIDEO:
		return "DVVIDEO"
	case CodecIDHUFFYUV:
		return "HUFFYUV"
	case CodecIDCYUV:
		return "CYUV"
	case CodecIDH264:
		return "H264"
	case CodecIDINDEO3:
		return "INDEO3"
	case CodecIDVP3:
		return "VP3"
	default:
		return "unknown"
	}
}

// FindDecoder Find a registered decoder with a matching codec ID.
//
// @param id AVCodecID of the requested decoder
// @return A decoder if one was found, NULL otherwise.
func FindDecoder(ctx context.Context, id CodecID) (Codec, error) {
	var zero Codec
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
		p := C.avcodec_find_decoder((C.enum_AVCodecID)(id))
		if p == nil {
			return zero, errors.Errorf("cannot find decoder %s(%d)", id.ToString(), int(id))
		}
		return Codec{
			p:  p,
			pp: &p,
		}, nil
	}
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
func PacketAlloc() (Packet, error) {
	p := C.av_packet_alloc()
	if p == nil {
		return Packet{}, errors.New("cannot allocate packet")
	}

	return Packet{
		p:  p,
		pp: &p,
	}, nil
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
func (c *Codec) AllocContext3(ctx context.Context) (CodecContext, error) {
	var zero CodecContext
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
		p := C.avcodec_alloc_context3(c.p)
		if p == nil {
			return zero, errors.New("cannot allocate context from codec")
		}
		return CodecContext{
			p:  p,
			pp: &p,
		}, nil
	}
}

// FreeContext Free the codec context and everything associated with it and write NULL to
// the provided pointer.
func (ctxt *CodecContext) FreeContext() {
	C.avcodec_free_context(ctxt.pp)
}

// SetEncodeParams sets the context's width, height, pixel format (pxlFmt), if it has b-frames and GOP size.
func (ctxt *CodecContext) SetEncodeParams(width, height int, pxlFmt PixelFormat, hasBFrames bool, gopSize int) {
	ctxt.p.width = C.int(width)
	ctxt.p.height = C.int(height)
	ctxt.p.bit_rate = bitrate
	ctxt.p.gop_size = C.int(gopSize)
	if hasBFrames {
		ctxt.p.has_b_frames = 1
	} else {
		ctxt.p.has_b_frames = 0
	}
	ctxt.p.pix_fmt = int32(pxlFmt)
	ctxt.p.rc_buffer_size = bitrate * bitrateDeviation
}

// SetFramerate sets the context's framerate
func (ctxt *CodecContext) SetFramerate(fps int) {
	ctxt.p.framerate.num = C.int(fps)
	ctxt.p.framerate.den = C.int(1)

	// timebase should be 1/framerate
	ctxt.p.time_base.num = C.int(1)
	ctxt.p.time_base.den = C.int(fps)
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
func (ctxt *CodecContext) Open2(ctx context.Context, c Codec) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrorFromCode(int(
			C.avcodec_open2(ctxt.p, c.p, nil),
		))
	}
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
func (ctxt *CodecContext) Close() int {
	return int(C.avcodec_close(ctxt.p))
}

// Unref Wipe the packet.
//
// Unreference the buffer referenced by the packet and reset the
// remaining packet fields to their default values.
//
// @param pkt The packet to be unreferenced.
func (p *Packet) Unref() {
	C.av_packet_unref(p.p)
}

// Free frees the packet
func (p *Packet) Free() {
	C.av_packet_free(p.pp)
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
func (ctxt *CodecContext) SendFrame(f Frame) int {
	return int(C.avcodec_send_frame(ctxt.p, f.p))
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
func (ctxt *CodecContext) ReceivePacket(a Packet) int {
	return int(C.avcodec_receive_packet(ctxt.p, a.p))
}

// Data returns the packet's data
func (p *Packet) Data() *uint8 {
	return (*uint8)(p.p.data)
}

// Size returns the packet size
func (p *Packet) Size() int {
	return int(p.p.size)
}

// EncoderIsAvailable returns true if the given encoder is available, false otherwise.
func EncoderIsAvailable(enc string) bool {
	// TODO: get context from args
	ctx := context.Background()

	// Quiet logging during function execution, but reset afterward.
	lvl := GetLevel()
	defer SetLevel(lvl)
	SetLevel(LogQuiet)

	codec, err := FindEncoderByName(enc)
	if err != nil {
		return false
	}

	context, err := codec.AllocContext3(ctx)
	if err != nil {
		return false
	}

	// Only need positive values
	context.SetEncodeParams(1, 1, YUV420P, false, 1)
	context.SetFramerate(1)

	return context.Open2(ctx, codec) == nil
}

// SendPacket Supply raw packet data as input to a decoder.
//
// Internally, this call will copy relevant AVCodecContext fields, which can
// influence decoding per-packet, and apply them when the packet is actually
// decoded. (For example AVCodecContext.skip_frame, which might direct the
// decoder to drop the frame contained by the packet sent with this function.)
//
// @warning The input buffer, avpkt->data must be AV_INPUT_BUFFER_PADDING_SIZE
//
//	larger than the actual read bytes because some optimized bitstream
//	readers read 32 or 64 bits at once and could read over the end.
//
// @note The AVCodecContext MUST have been opened with @ref avcodec_open2()
//
//	before packets may be fed to the decoder.
//
// @param avctx codec context
// @param[in] avpkt The input AVPacket. Usually, this will be a single video
//
//	frame, or several complete audio frames.
//	Ownership of the packet remains with the caller, and the
//	decoder will not write to the packet. The decoder may create
//	a reference to the packet data (or copy it if the packet is
//	not reference-counted).
//	Unlike with older APIs, the packet is always fully consumed,
//	and if it contains multiple frames (e.g. some audio codecs),
//	will require you to call avcodec_receive_frame() multiple
//	times afterwards before you can send a new packet.
//	It can be NULL (or an AVPacket with data set to NULL and
//	size set to 0); in this case, it is considered a flush
//	packet, which signals the end of the stream. Sending the
//	first flush packet will return success. Subsequent ones are
//	unnecessary and will return AVERROR_EOF. If the decoder
//	still has frames buffered, it will return them after sending
//	a flush packet.
//
// @retval 0                 success
// @retval AVERROR(EAGAIN)   input is not accepted in the current state - user
//
//	must read output with avcodec_receive_frame() (once
//	all output is read, the packet should be resent,
//	and the call will not fail with EAGAIN).
//
// @retval AVERROR_EOF       the decoder has been flushed, and no new packets can be
//
//	sent to it (also returned if more than 1 flush
//	packet is sent)
//
// @retval AVERROR(EINVAL)   codec not opened, it is an encoder, or requires flush
// @retval AVERROR(ENOMEM)   failed to add packet to internal queue, or similar
// @retval "another negative error code" legitimate decoding errors
func (ctxt *CodecContext) SendPacket(pkt Packet) int {
	return int(C.avcodec_send_packet(ctxt.p, pkt.p))
}

// FlushBuffers Reset the internal codec state / flush internal buffers. Should be called
// e.g. when seeking or when switching to a different stream.
//
// @note for decoders, this function just releases any references the decoder
// might keep internally, but the caller's references remain valid.
//
// @note for encoders, this function will only do something if the encoder
// declares support for AV_CODEC_CAP_ENCODER_FLUSH. When called, the encoder
// will drain any remaining packets, and can then be re-used for a different
// stream (as opposed to sending a null frame which will leave the encoder
// in a permanent EOF state after draining). This can be desirable if the
// cost of tearing down and replacing the encoder instance is high.
func (ctxt *CodecContext) FlushBuffers() {
	C.avcodec_flush_buffers(ctxt.p)
}

// ReceiveFrame Return decoded output data from a decoder or encoder (when the
// @ref AV_CODEC_FLAG_RECON_FRAME flag is used).
//
// @param avctx codec context
// @param frame This will be set to a reference-counted video or audio
//
//	frame (depending on the decoder type) allocated by the
//	codec. Note that the function will always call
//	av_frame_unref(frame) before doing anything else.
//
// @retval 0                success, a frame was returned
// @retval AVERROR(EAGAIN)  output is not available in this state - user must
//
//	try to send new input
//
// @retval AVERROR_EOF      the codec has been fully flushed, and there will be
//
//	no more output frames
//
// @retval AVERROR(EINVAL)  codec not opened, or it is an encoder without the
//
//	@ref AV_CODEC_FLAG_RECON_FRAME flag enabled
//
// @retval "other negative error code" legitimate decoding errors
func (ctxt *CodecContext) ReceiveFrame(ctx context.Context, frame Frame) int {
	select {
	case <-ctx.Done():
		return C.AVERROR_EXIT
	default:
		return int(C.avcodec_receive_frame(ctxt.p, frame.p))
	}
}

// PixFmt return the pixel format
func (ctxt *CodecContext) PixFmt() int {
	return int(ctxt.p.pix_fmt)
}

func toSlice(buf unsafe.Pointer, size int) []uint8 {
	return unsafe.Slice((*uint8)(buf), size)
}

// GetSubsampleRatio returns the subsample ratio
func (f *Frame) GetSubsampleRatio() image.YCbCrSubsampleRatio {
	switch f.p.format {
	case C.AV_PIX_FMT_YUV444P:
		return image.YCbCrSubsampleRatio444
	case C.AV_PIX_FMT_YUV422P:
		return image.YCbCrSubsampleRatio422
	case C.AV_PIX_FMT_YUV440P10:
		return image.YCbCrSubsampleRatio440
	default:
		return image.YCbCrSubsampleRatio420
	}
}

// ToImage returns an image.Image created from the frame.
func (f *Frame) ToImage() image.Image {
	w := int(f.p.width)
	h := int(f.p.height)
	ys := int(f.p.linesize[0])
	cs := int(f.p.linesize[1])

	img := image.YCbCr{
		Y:              toSlice(unsafe.Pointer(f.p.data[0]), ys*h),
		Cb:             toSlice(unsafe.Pointer(f.p.data[1]), cs*h/2),
		Cr:             toSlice(unsafe.Pointer(f.p.data[2]), cs*h/2),
		YStride:        ys,
		CStride:        cs,
		SubsampleRatio: f.GetSubsampleRatio(),
		Rect:           image.Rect(0, 0, w, h),
	}

	return &img
}

// StreamIndex returns the steam index
func (p *Packet) StreamIndex() int {
	return int(p.p.stream_index)
}
