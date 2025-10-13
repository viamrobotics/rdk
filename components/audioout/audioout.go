// Package audioout defines an audioout component
package audioout

import (
	"context"

	pb "go.viam.com/api/component/audioout/v1"
	"go.viam.com/rdk/resource"
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

// An AudioInput is a resource that can output audio.
type AudioOut interface {
	resource.Resource
	Play(ctx context.Context, data []byte, AudioInfo info) error
}
