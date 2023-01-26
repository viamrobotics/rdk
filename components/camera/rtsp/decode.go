package rtsp

import (
	"bytes"
	"image"
	"image/jpeg"

	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/pion/rtp"
)

type decoder func(pkt *rtp.Packet) (image.Image, error)

func mjpegDecoding() (*format.MJPEG, decoder) {
	var mjpeg format.MJPEG
	// get the RTP->MJPEG decoder
	rtpDec := mjpeg.CreateDecoder()
	mjpegDecoder := func(pkt *rtp.Packet) (image.Image, error) {
		encoded, _, err := rtpDec.Decode(pkt)
		if err != nil {
			return nil, err
		}
		return jpeg.Decode(bytes.NewReader(encoded))
	}
	return &mjpeg, mjpegDecoder
}
