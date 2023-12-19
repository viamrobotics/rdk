//go:build cgo && !android

package server

import (
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	// TODO(RSDK-5916): support v4l2m2m codec on arm64/rpi
	streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	return streamConfig
}
