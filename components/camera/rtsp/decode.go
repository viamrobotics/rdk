package rtsp

import (
	"bytes"
	"image"

	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/pion/rtp"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

type decoder func(pkt *rtp.Packet) (image.Image, error)

func mjpegDecoding() (*format.MJPEG, decoder) {
	var mjpeg format.MJPEG
	// get the RTP->MJPEG decoder
	rtpDec := mjpeg.CreateDecoder()
	mjpegDecoder := func(pkt *rtp.Packet) (image.Image, error) {
		encoded, _, err := rtpDec.Decode(pkt)
		if err != nil {
			return nil, errors.Wrap(err, "rtp to mjpeg decoding failed")
		}
		return rimage.DecodeJPEG(bytes.NewReader(encoded))
	}
	return &mjpeg, mjpegDecoder
}
