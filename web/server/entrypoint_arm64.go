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
	if avcodec.findencoderbyname(h264.V4l2m2m) != nil {
		streamconfig.videoencoderfactory = h264.newencoderfactory()
	} else {
		streamconfig.videoencoderfactory = x264.newencoderfactory()
	}
	streamconfig.audioencoderfactory = opus.newencoderfactory()
	return streamconfig
}
