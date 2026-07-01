// Package button contains a gRPC based button service server.
package button

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/button/v1"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the ButtonService from button.proto.
type serviceServer struct {
	pb.UnimplementedButtonServiceServer
	coll resource.APIResourceGetter[Button]
}

// NewRPCServiceServer constructs an gripper gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Button], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// Pushes a button.
func (s *serviceServer) Push(ctx context.Context, req *pb.PushRequest) (*pb.PushResponse, error) {
	button, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.PushResponse{}, errtrace.Wrap(button.Push(ctx, req.Extra.AsMap()))
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	button, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.DoFromResourceServer(ctx, button, req))
}

// GetStatus returns the status of the button.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.GetStatusFromResourceServer(ctx, res, req))
}
