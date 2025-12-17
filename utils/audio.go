package utils

import commonpb "go.viam.com/api/common/v1"

// Common audio codec constants.
const (
	CodecPCM16      = "pcm16"
	CodecPCM32      = "pcm32"
	CodecPCM32Float = "pcm32_float"
	CodecOpus       = "opus"
	CodecAAC        = "aac"
	CodecMP3        = "mp3"
	CodecFLAC       = "flac"
)

// Properties defines properties of an audio device.
type Properties struct {
	SupportedCodecs []string
	SampleRateHz    int32
	NumChannels     int32
}

// AudioInfo defines information about audio data.
type AudioInfo struct {
	Codec        string
	SampleRateHz int32
	NumChannels  int32
}

// AudioInfoPBToStruct converts a protobuf audioinfo struct to the golang struct.
func AudioInfoPBToStruct(pb *commonpb.AudioInfo) *AudioInfo {
	return &AudioInfo{
		Codec:        pb.Codec,
		SampleRateHz: pb.SampleRateHz,
		NumChannels:  pb.NumChannels,
	}
}

// AudioInfoStructToPb converts a go audioinfo struct to the protobuf struct.
func AudioInfoStructToPb(info *AudioInfo) *commonpb.AudioInfo {
	return &commonpb.AudioInfo{
		Codec:        info.Codec,
		SampleRateHz: info.SampleRateHz,
		NumChannels:  info.NumChannels,
	}
}
