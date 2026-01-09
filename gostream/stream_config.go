package gostream

import (
	"go.viam.com/rdk/gostream/codec"
)

// A StreamConfig describes how a Stream should be managed.
type StreamConfig struct {
	Name                string
	VideoEncoderFactory codec.VideoEncoderFactory

	// TargetFrameRate will hint to the stream to try to maintain this frame rate.
	TargetFrameRate int
}
