// Package video contains the video service implementation.
package video

import (
	"context"
	"time"

	servicepb "go.viam.com/api/service/video/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterVideoServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.VideoService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// Chunk defines a chunk of video data.
type Chunk struct {
	Data      []byte
	Container string
	RequestID string
}

// Service is the interface for a video service.
type Service interface {
	resource.Resource
	GetVideo(
		ctx context.Context,
		startTime, endTime time.Time,
		videoCodec, videoContainer string,
		extra map[string]interface{},
	) (chan *Chunk, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = "video"

// API is a variable that identifies the video service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named video typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Deprecated: FromRobot is a helper for getting the named video service from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named video service from a collection of dependencies.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// FromProvider is a helper for getting the named video service
// from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Service, error) {
	return resource.FromProvider[Service](provider, Named(name))
}
