// Package audioin defines an audioin component
package audioin

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioin/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[AudioIn]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterAudioInServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.AudioInService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the AudioIn resource subtype string.
const SubtypeName = "audio_in"

// API is a variable that identifies the AudioIn resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named AudioIn's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Properties defines properties of an audio in device.
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

// AudioChunk defines a chunk of audio data.
type AudioChunk struct {
	AudioData                 []byte
	AudioInfo                 *AudioInfo
	Sequence                  int32
	StartTimestampNanoseconds int64
	EndTimestampNanoseconds   int64
	RequestID                 string
}

// AudioIn defines an audioin component.
type AudioIn interface {
	resource.Resource
	GetAudio(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64, extra map[string]interface{}) (
		chan *AudioChunk, error)
	Properties(ctx context.Context, extra map[string]interface{}) (Properties, error)
}

// FromProvider is a helper for getting the named Board from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (AudioIn, error) {
	return resource.FromProvider[AudioIn](provider, Named(name))
}

// FromRobot is a helper for getting the named AudioIn from the given Robot.
func FromRobot(r robot.Robot, name string) (AudioIn, error) {
	return robot.ResourceFromRobot[AudioIn](r, Named(name))
}

// NamesFromRobot is a helper for getting all AudioIn names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

func audioChunkToPb(chunk *AudioChunk) *pb.AudioChunk {
	if chunk == nil {
		return nil
	}

	var info *commonpb.AudioInfo
	if chunk.AudioInfo != nil {
		info = &commonpb.AudioInfo{
			Codec:        chunk.AudioInfo.Codec,
			SampleRateHz: chunk.AudioInfo.SampleRateHz,
			NumChannels:  chunk.AudioInfo.NumChannels,
		}
	}

	return &pb.AudioChunk{
		AudioData:                 chunk.AudioData,
		AudioInfo:                 info,
		StartTimestampNanoseconds: chunk.StartTimestampNanoseconds,
		EndTimestampNanoseconds:   chunk.EndTimestampNanoseconds,
		Sequence:                  chunk.Sequence,
	}
}

func audioInfoPBToStruct(pb *commonpb.AudioInfo) *AudioInfo {
	return &AudioInfo{
		Codec:        pb.Codec,
		SampleRateHz: pb.SampleRateHz,
		NumChannels:  pb.NumChannels,
	}
}
