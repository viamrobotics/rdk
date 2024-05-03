package server

import (
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	return streamConfig
}
