//go:build arm64 && cgo

package server

import (
	"strings"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/h264"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
	"go.viam.com/rdk/utils"
)

var streamConfig gostream.StreamConfig

func init() {
	if osInfo, err := utils.DetectOSInformation(); err == nil && strings.Contains(osInfo.Device, "Raspberry Pi") {
		streamConfig.VideoEncoderFactory = h264.NewEncoderFactory()
	} else {
		streamConfig.VideoEncoderFactory = x264.NewEncoderFactory()
	}
}

func makeStreamConfig() gostream.StreamConfig {
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	return streamConfig
}
