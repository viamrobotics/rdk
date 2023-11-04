//go:build cgo && !android

package server

import (
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/h264"
	"go.viam.com/rdk/gostream/codec/h264/ffmpeg/avcodec"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	if avcodec.FindEncoderByName(h264.V4l2m2m) != nil {
		streamConfig.VideoEncoderFactory = h264.NewEncoderFactory()
	} else {
		streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	}
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	return streamConfig
}
