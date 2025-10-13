// Package audioout defines an audioout component
package audioout

import (
	"context"

	commonpb "go.viam.com/api/common/v1"

	pb "go.viam.com/api/component/audioout/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[AudioOut]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterAudioOutServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.AudioOutService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the audio out resource subtype string.
const SubtypeName = "audio_out"

// API is a variable that identifies the audio input resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named audio inputs's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Properties defines properties of an audio out device.
type Properties struct {
	SupportedCodecs []string
	SampleRate      int32
	NumChannels     int32
}

// AudioInfo defines information about audio data.
type AudioInfo struct {
	codec       string
	sampleRate  int32
	numChannels int32
}

// An AudioInput is a resource that can output audio.
type AudioOut interface {
	resource.Resource
	Play(ctx context.Context, data []byte, info *AudioInfo, extra map[string]interface{}) error
	Properties(ctx context.Context, extra map[string]interface{}) (Properties, error)
}

// FromDependencies is a helper for getting the named AudioOut from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (AudioOut, error) {
	return resource.FromDependencies[AudioOut](deps, Named(name))
}

// FromRobot is a helper for getting the named AudioOutfrom the given Robot.
func FromRobot(r robot.Robot, name string) (AudioOut, error) {
	return robot.ResourceFromRobot[AudioOut](r, Named(name))
}

// NamesFromRobot is a helper for getting all AudioIn names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

func audioInfoPBToStruct(pb *commonpb.AudioInfo) *AudioInfo {
	return &AudioInfo{
		codec:       pb.Codec,
		sampleRate:  pb.SampleRate,
		numChannels: pb.NumChannels,
	}
}

func audioInfoStructToPb(info *AudioInfo) *commonpb.AudioInfo {
	return &commonpb.AudioInfo{
		codec:       info.codec,
		sampleRate:  info.sampleRate,
		numChannels: info.numChannels,
	}
}
