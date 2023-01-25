//go:build linux || darwin

package server

import (
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/opus"
	"github.com/edaniels/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	return streamConfig
}
