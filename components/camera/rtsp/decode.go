package rtsp

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"unsafe"

	"github.com/pkg/errors"

	"go.viam.com/rdk/gostream/ffmpeg"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// Decoder the decoder created for the RTSP stream.
type Decoder struct {
	codec       ffmpeg.Codec
	context     ffmpeg.CodecContext
	fmtCxt      ffmpeg.FormatContext
	videoStream int
	cancelCtx   context.Context
	cancelFunc  func()
}

// NewDecoder returns a new decoder.
func NewDecoder(ctx context.Context, addr string) (*Decoder, error) {
	dec := &Decoder{}
	dec.fmtCxt = ffmpeg.AllocFormatContext()
	dec.cancelCtx, dec.cancelFunc = context.WithCancel(context.Background())

	dict, err := ffmpeg.NewDictionary(ctx, ffmpeg.DictionaryEntries{
		// Use `ffmpeg -h full` for a complete list of options
		"rtsp_transport": "tcp",
		"scan_all_pmts":  "1",
		"sdp_flags":      "rtcp_to_source",
		"buffer_size":    "-1",
		"timeout":        "-1",
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dictionary")
	}
	defer dict.Free()

	if err := dec.fmtCxt.OpenInput(ctx, addr, dict); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("cannot open input for address '%s'", addr))
	}

	if err := dec.fmtCxt.FindStreamInfo(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot find stream information")
	}

	if dec.videoStream, err = dec.fmtCxt.FindBestVideoStream(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot find the best video stream for use")
	}

	params, err := dec.fmtCxt.CodecParameters(ctx, dec.videoStream)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get codec parameters")
	}

	id, err := params.CodecID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get codec ID from parameters")
	}

	if dec.codec, err = ffmpeg.FindDecoder(ctx, id); err != nil {
		return nil, errors.Wrap(err, "cannot find decoder")
	}

	if dec.context, err = dec.codec.AllocContext3(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot allocate context from codec")
	}

	if err := dec.fmtCxt.ParamsToContext(ctx, dec.context, dec.videoStream); err != nil {
		return nil, errors.Wrap(err, "cannot convert parameters to context")
	}

	if err := dec.context.Open2(ctx, dec.codec); err != nil {
		return nil, errors.Wrap(err, "cannot open decoder")
	}

	dec.fmtCxt.ReadPlay(ctx)
	dec.context.FlushBuffers()

	return dec, nil
}

// Close closes the decoder.
func (dec *Decoder) Close() {
	dec.cancelFunc()
	dec.context.Close()
	dec.fmtCxt.Close()
}

// Decode retrieves and (optionally for H264) decodes the next frame returned from the RTSP stream.
func (dec *Decoder) Decode(ctx context.Context) (image.Image, error) {
	// See https://ffmpeg.org/doxygen/trunk/api-band-test_8c_source.html#l00163
	frame, err := ffmpeg.FrameAlloc()
	if err != nil {
		return nil, err
	}
	defer frame.Free()

	pkt, err := ffmpeg.PacketAlloc()
	if err != nil {
		return nil, err
	}
	defer pkt.Free()

	for {
		if err := dec.fmtCxt.ReadFrame(ctx, pkt); err == nil && pkt.StreamIndex() != dec.videoStream {
			pkt.Unref()
			continue
		}

		switch pixFmt := dec.context.PixFmt(); pixFmt {
		case ffmpeg.PixFmtYUV420P, ffmpeg.PixFmtYUV422P, ffmpeg.PixFmtYUV444P:
			result := dec.context.SendPacket(pkt)
			pkt.Unref()
			if result < 0 {
				return nil, errors.Wrap(ffmpeg.ErrorFromCode(result), "cannot send packet")
			}

			if result = dec.context.ReceiveFrame(ctx, frame); result == ffmpeg.EOF {
				return nil, errors.Wrap(ffmpeg.ErrorFromCode(result), "cannot receive frame")
			} else if result == ffmpeg.EAGAIN {
				continue
			} else if result < 0 {
				return nil, errors.Wrap(ffmpeg.ErrorFromCode(result), "error decoding frame")
			}

			img := frame.ToImage()
			frame.Unref()
			return img, nil
		case ffmpeg.PixFmtYUVJ420P, ffmpeg.PixFmtYUVJ422P, ffmpeg.PixFmtYUVJ444P:
			b := bytes.Buffer{}
			b.Write(unsafe.Slice(pkt.Data(), pkt.Size())) // we must copy bytes before pkt.Unref or pkt.Free
			// TODO: ideally, we'd do something similar for H264, storing the raw bytes in a lazy image and decoding
			// them client-side.
			return rimage.NewLazyEncodedImage(b.Bytes(), utils.MimeTypeJPEG), nil
		default:
			return nil, errors.Errorf("unknown pixel format (%d)", pixFmt)
		}
	}
}
