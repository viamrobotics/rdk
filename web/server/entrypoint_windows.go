//go:build windows

package server

import (
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
)

func makeStreamConfig() gostream.StreamConfig {
	var streamConfig gostream.StreamConfig
	// TODO(RSDK-1771): support video on windows
	streamConfig.AudioEncoderFactory = opus.NewEncoderFactory()
	return streamConfig
}
