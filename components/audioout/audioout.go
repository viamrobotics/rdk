// Package audioout defines an audioout component
package audioout

import (
	"context"

	pb "go.viam.com/api/component/audioout/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
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

// API is a variable that identifies the audio out's resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named audio output's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// An AudioOut is a resource that can output audio.
type AudioOut interface {
	resource.Resource
	Play(ctx context.Context, data []byte, info *utils.AudioInfo, extra map[string]interface{}) error
	Properties(ctx context.Context, extra map[string]interface{}) (utils.Properties, error)
}

// FromProvider is a helper for getting the named Board from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (AudioOut, error) {
	return resource.FromProvider[AudioOut](provider, Named(name))
}

// NamesFromRobot is a helper for getting all AudioIn names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}
