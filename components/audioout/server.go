package audioout

import (
	pb "go.viam.com/api/component/audioout/v1"

	"go.viam.com/rdk/resource"
)

// serviceServer implements the ButtonService from button.proto.
type serviceServer struct {
	pb.UnimplementedAudioOutServiceServer
	coll resource.APIResourceGetter[AudioOut]
}

// NewRPCServiceServer constructs an gripper gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[AudioOut]) interface{} {
	return &serviceServer{coll: coll}
}
