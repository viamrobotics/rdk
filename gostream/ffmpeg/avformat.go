//go:build cgo && !android

package ffmpeg

//#include <libavformat/avformat.h>
//#include <libavutil/avutil.h>
//#include <libavcodec/avcodec.h>
// static AVCodecParameters * av_format_context_get_codec_par(AVFormatContext* fmt_ctx, int video_stream) {
// 		return fmt_ctx->streams[video_stream]->codecpar;
// }
import "C"

import (
	"context"
	"fmt"
	"strconv"
	"unsafe"
)

type (
	// Stream the AVStream
	Stream C.struct_AVStream
	// FormatContext AVFormatContext
	FormatContext struct {
		p  *C.struct_AVFormatContext
		pp **C.struct_AVFormatContext
	}
	// Dictionary an AVDictionary
	Dictionary struct {
		p  *C.struct_AVDictionary
		pp **C.struct_AVDictionary
	}
)

// DictionaryEntries entries to be added to an ffmpeg.Dictionary
type DictionaryEntries map[string]string

// NewDictionary creates a new dictionary
func NewDictionary(ctx context.Context, e DictionaryEntries) (Dictionary, error) {
	var d *C.struct_AVDictionary
	for k, v := range e {
		select {
		case <-ctx.Done():
			return Dictionary{}, ctx.Err()
		default:
			if i, err := strconv.Atoi(v); err == nil {
				if err := ErrorFromCode(int(C.av_dict_set_int(&d, C.CString(k), C.long(i), 0))); err != nil { //nolint: gocritic
					return Dictionary{}, err
				}
			} else {
				if err := ErrorFromCode(int(C.av_dict_set(&d, C.CString(k), C.CString(v), C.AV_DICT_DONT_STRDUP_KEY))); err != nil { //nolint: gocritic
					return Dictionary{}, err
				}
			}
		}
	}

	return Dictionary{
		p:  d,
		pp: &d,
	}, nil
}

// Free all the memory allocated for an AVDictionary struct
// and all keys and values.
func (d *Dictionary) Free() {
	if d != nil {
		C.av_dict_free(d.pp)
	}
}

// String converts the Dictionary to a string
func (d *Dictionary) String() string {
	m := map[string]string{}
	var e *C.struct_AVDictionaryEntry
	for e = C.av_dict_iterate(d.p, e); e != nil; e = C.av_dict_iterate(d.p, e) {
		m[C.GoString(e.key)] = C.GoString(e.value)
	}
	return fmt.Sprint(m)
}

// NetworkInit Do global initialization of network libraries. This is optional,
// and not recommended anymore.
//
// This functions only exists to work around thread-safety issues
// with older GnuTLS or OpenSSL libraries. If libavformat is linked
// to newer versions of those libraries, or if you do not use them,
// calling this function is unnecessary. Otherwise, you need to call
// this function before any other threads using them are started.
//
// This function will be deprecated once support for older GnuTLS and
// OpenSSL libraries is removed, and this function has no purpose
// anymore.
func NetworkInit(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		C.avformat_network_init()
	}
}

// AllocFormatContext Format I/O context.
// New fields can be added to the end with minor version bumps.
// Removal, reordering and changes to existing fields require a major
// version bump.
// sizeof(AVFormatContext) must not be used outside libav*, use
// avformat_alloc_context() to create an AVFormatContext.
//
// Fields can be accessed through AVOptions (av_opt*),
// the name string used matches the associated command line parameter name and
// can be found in libavformat/options_table.h.
// The AVOption/command line parameter names differ in some cases from the C
// structure field names for historic reasons or brevity.
func AllocFormatContext() FormatContext {
	c := C.avformat_alloc_context()
	return FormatContext{
		p:  c,
		pp: &c,
	}
}

// OpenInput Open an input stream and read the header. The codecs are not opened.
// The stream must be closed with avformat_close_input().
//
// @param ps       Pointer to user-supplied AVFormatContext (allocated by
//
//	avformat_alloc_context). May be a pointer to NULL, in
//	which case an AVFormatContext is allocated by this
//	function and written into ps.
//	Note that a user-supplied AVFormatContext will be freed
//	on failure.
//
// @param url      URL of the stream to open.
// @param fmt      If non-NULL, this parameter forces a specific input format.
//
//	Otherwise the format is autodetected.
//
// @param options  A dictionary filled with AVFormatContext and demuxer-private
//
//	options.
//	On return this parameter will be destroyed and replaced with
//	a dict containing options that were not found. May be NULL.
//
// @return 0 on success, a negative AVERROR on failure.
//
// @note If you want to use custom IO, preallocate the format context and set its pb field.
func (ic *FormatContext) OpenInput(ctx context.Context, addr string, d Dictionary) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrorFromCode(int(
			C.avformat_open_input(ic.pp,
				C.CString(addr),
				nil,
				d.pp,
			),
		))
	}
}

// Close an opened input AVFormatContext. Free it and all its contents
// and set *s to NULL.
func (ic *FormatContext) Close() {
	if ic != nil {
		C.avformat_close_input(ic.pp)
	}
}

// FindStreamInfo Read packets of a media file to get stream information. This
// is useful for file formats with no headers such as MPEG. This
// function also computes the real framerate in case of MPEG-2 repeat
// frame mode.
// The logical file position is not changed by this function;
// examined packets may be buffered for later processing.
//
// @param ic media file handle
// @param options  If non-NULL, an ic.nb_streams long array of pointers to
//
//	dictionaries, where i-th member contains options for
//	codec corresponding to i-th stream.
//	On return each dictionary will be filled with options that were not found.
//
// @return >=0 if OK, AVERROR_xxx on error
//
// @note this function isn't guaranteed to open all the codecs, so
//
//	options being non-empty at return is a perfectly normal behavior.
//
// @todo Let the user decide somehow what information is needed so that
//
//	we do not waste time getting stuff the user does not need.
func (ic *FormatContext) FindStreamInfo(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrorFromCode(int(C.avformat_find_stream_info(ic.p, nil)))
	}
}

// FindBestVideoStream Find the "best" stream in the file.
// The best stream is determined according to various heuristics as the most
// likely to be what the user expects.
// If the decoder parameter is non-NULL, av_find_best_stream will find the
// default decoder for the stream's codec; streams for which no decoder can
// be found are ignored.
//
// @param ic                media file handle
// @param type              stream type: video, audio, subtitles, etc.
// @param wanted_stream_nb  user-requested stream number,
//
//	or -1 for automatic selection
//
// @param related_stream    try to find a stream related (eg. in the same
//
//	program) to this one, or -1 if none
//
// @param decoder_ret       if non-NULL, returns the decoder for the
//
//	selected stream
//
// @param flags             flags; none are currently defined
//
// @return  the non-negative stream number in case of success,
//
//	AVERROR_STREAM_NOT_FOUND if no stream with the requested type
//	could be found,
//	AVERROR_DECODER_NOT_FOUND if streams were found but no decoder
//
// @note  If av_find_best_stream returns successfully and decoder_ret is not
//
//	NULL, then *decoder_ret is guaranteed to be set to a valid AVCodec.
func (ic *FormatContext) FindBestVideoStream(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		stream := C.av_find_best_stream(
			ic.p,
			(C.enum_AVMediaType)(AvmediaTypeVideo),
			-1, -1, nil, 0,
		)
		return int(stream), nil
	}
}

// Parameters the AVCodecParameters
type Parameters C.struct_AVCodecParameters

// CodecParameters get codec parameters from context
func (ic *FormatContext) CodecParameters(ctx context.Context, stream int) (*Parameters, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		par := C.av_format_context_get_codec_par(ic.p, C.int(stream))
		return (*Parameters)(par), nil
	}
}

// CodecID returns the codec ID from the parameters
func (p *Parameters) CodecID(ctx context.Context) (CodecID, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		return (CodecID)((*C.struct_AVCodecParameters)(p).codec_id), nil
	}
}

// ReadFrame Return the next frame of a stream.
// This function returns what is stored in the file, and does not validate
// that what is there are valid frames for the decoder. It will split what is
// stored in the file into frames and return one for each call. It will not
// omit invalid data between valid frames so as to give the decoder the maximum
// information possible for decoding.
//
// On success, the returned packet is reference-counted (pkt->buf is set) and
// valid indefinitely. The packet must be freed with av_packet_unref() when
// it is no longer needed. For video, the packet contains exactly one frame.
// For audio, it contains an integer number of frames if each frame has
// a known fixed size (e.g. PCM or ADPCM data). If the audio frames have
// a variable size (e.g. MPEG audio), then it contains one frame.
//
// pkt->pts, pkt->dts and pkt->duration are always set to correct
// values in AVStream.time_base units (and guessed if the format cannot
// provide them). pkt->pts can be AV_NOPTS_VALUE if the video format
// has B-frames, so it is better to rely on pkt->dts if you do not
// decompress the payload.
//
// @return 0 if OK, < 0 on error or end of file. On error, pkt will be blank
//
//	(as if it came from av_packet_alloc()).
//
// @note pkt will be initialized, so it may be uninitialized, but it must not
//
//	contain data that needs to be freed.
func (ic *FormatContext) ReadFrame(ctx context.Context, pkt Packet) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		ret := C.av_read_frame(
			ic.p,
			pkt.p,
		)
		return ErrorFromCode(int(ret))
	}
}

// ReadPlay Start playing a network-based stream (e.g. RTSP stream) at the
// current position.
func (ic *FormatContext) ReadPlay(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		C.av_read_play(ic.p)
	}
}

// ParamsToContext Fill the codec context based on the values from the supplied codec
// parameters. Any allocated fields in codec that have a corresponding field in
// par are freed and replaced with duplicates of the corresponding field in par.
// Fields in codec that do not have a counterpart in par are not touched.
//
// @return >= 0 on success, a negative AVERROR code on failure.
func (ic *FormatContext) ParamsToContext(ctx context.Context, ctxt CodecContext, i int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	length := ic.p.nb_streams
	streams := (*[1 << 30]*Stream)(unsafe.Pointer(ic.p.streams))[:length:length]
	codecPar := streams[i].codecpar
	return ErrorFromCode(int(
		C.avcodec_parameters_to_context(ctxt.p, codecPar),
	))
}
