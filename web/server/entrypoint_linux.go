//go:build cgo && linux

package server

import (
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/h264"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig

	// Attempt to create a new encoder with hardcoded parameters
	// to check if V4l2m2m codec is supported.
	width, height, keyFrameInterval := 1920, 1080, 30
	_, err := h264.NewEncoder(width, height, keyFrameInterval, nil)
	if err != nil {
		streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	} else {
		streamConfig.VideoEncoderFactory = h264.NewEncoderFactory()
	}

	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	return streamConfig
}
