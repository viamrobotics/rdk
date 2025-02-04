package opus

import (
	"time"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
)

// DefaultStreamConfig configures Opus as the audio encoder for a stream.
var DefaultStreamConfig gostream.StreamConfig

func init() {
	DefaultStreamConfig.AudioEncoderFactory = NewEncoderFactory()
}

// NewEncoderFactory returns an Opus audio encoder factory.
func NewEncoderFactory() codec.AudioEncoderFactory {
	return &factory{}
}

type factory struct{}

func (f *factory) New(sampleRate, channelCount int, latency time.Duration, logger logging.Logger) (codec.AudioEncoder, error) {
	return NewEncoder(sampleRate, channelCount, latency, logger)
}

func (f *factory) MIMEType() string {
	return "audio/opus"
}
