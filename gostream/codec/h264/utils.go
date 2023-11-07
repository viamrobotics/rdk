//go:build cgo && arm64 && !android

package h264

import (
	"github.com/edaniels/golog"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/gostream/codec/h264/ffmpeg/avcodec"
)

// DefaultStreamConfig configures h264 as the encoder for a stream.
var DefaultStreamConfig gostream.StreamConfig

func init() {
	avcodec.RegisterAll()
	DefaultStreamConfig.VideoEncoderFactory = NewEncoderFactory()
}

// NewEncoderFactory returns an h264 encoder factory.
func NewEncoderFactory() codec.VideoEncoderFactory {
	return &factory{}
}

type factory struct{}

func (f *factory) New(width, height, keyFrameInterval int, logger golog.Logger) (codec.VideoEncoder, error) {
	return NewEncoder(width, height, keyFrameInterval, logger)
}

func (f *factory) MIMEType() string {
	return "video/H264"
}
