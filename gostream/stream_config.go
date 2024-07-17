package gostream

import (
	"go.viam.com/rdk/logging"

	"go.viam.com/rdk/gostream/codec"
)

// A StreamConfig describes how a Stream should be managed.
type StreamConfig struct {
	Name                string
	VideoEncoderFactory codec.VideoEncoderFactory
	AudioEncoderFactory codec.AudioEncoderFactory

	// TargetFrameRate will hint to the stream to try to maintain this frame rate.
	TargetFrameRate int

	Logger logging.Logger
}
