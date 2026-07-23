package x264

import (
	"math"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
)

const (
	encodeCompressionRatio = 0.15 // bits per pixel when encoded
	// For very small resolutions, we need to ensure that the vbv buffer size is large enough to
	// handle frame bursts. This is the minimum bitrate that we can use without causing the encoder
	// to spew out warnings about the buffer size being too small.
	minBitrate = 300_000 // 300kbps
	// This encoder is used exclusively for WebRTC delivery, which typically routes
	// through TURN relays or over Wi-Fi where sustained bitrates above ~3Mbps cause
	// packet loss that fragments H.264 keyframes and cascades into unrecoverable
	// decode failures.
	maxBitrate = 3_000_000 // 3Mbps
)

// DefaultStreamConfig configures x264 as the encoder for a stream.
var DefaultStreamConfig gostream.StreamConfig

func init() {
	DefaultStreamConfig.VideoEncoderFactory = NewEncoderFactory()
}

// NewEncoderFactory returns an x264 encoder factory.
func NewEncoderFactory() codec.VideoEncoderFactory {
	return &factory{}
}

type factory struct{}

func (f *factory) New(width, height, keyFrameInterval int, logger logging.Logger) (codec.VideoEncoder, error) {
	return NewEncoder(width, height, keyFrameInterval, logger)
}

func (f *factory) MIMEType() string {
	return "video/H264"
}

// calcBitrateFromResolution calculates the bitrate based on the given resolution and framerate.
func calcBitrateFromResolution(width, height int, framerate float32) int {
	bitrate := float32(width) * float32(height) * framerate * encodeCompressionRatio
	// Round up to the nearest integer value.
	bitrate = float32(math.Ceil(float64(bitrate)))
	// This accounts for zero bitrates too.
	if bitrate < minBitrate {
		return minBitrate
	}
	if bitrate > maxBitrate {
		return maxBitrate
	}
	return int(bitrate)
}
