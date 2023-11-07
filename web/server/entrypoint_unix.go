//go:build (linux || darwin) && !no_cgo && (!arm64 || android)

package server

import (
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	return streamConfig
}
