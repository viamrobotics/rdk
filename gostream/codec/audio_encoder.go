package codec

import (
	"context"
	"time"

	"github.com/pion/mediadevices/pkg/wave"

	"go.viam.com/rdk/logging"
)

// An AudioEncoder is anything that can encode audo chunks into bytes. This means that
// the encoder must follow some type of format dictated by a type (see AudioEncoderFactory.MimeType).
// An encoder that produces bytes of different encoding formats per call is invalid.
type AudioEncoder interface {
	Encode(ctx context.Context, chunk wave.Audio) ([]byte, bool, error)
	Close()
}

// An AudioEncoderFactory produces AudioEncoders and provides information about the underlying encoder itself.
type AudioEncoderFactory interface {
	New(sampleRate, channelCount int, latency time.Duration, logger logging.Logger) (AudioEncoder, error)
	MIMEType() string
}
