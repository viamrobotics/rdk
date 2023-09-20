//go:build (linux || darwin) && !no_cgo

package server

import (
	"github.com/viamrobotics/gostream"
	"github.com/viamrobotics/gostream/codec/opus"
	"github.com/viamrobotics/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	return streamConfig
}
