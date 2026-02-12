// Package servo contains a gRPC based servo service server
package servo

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/servo/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedServoServiceServer
	coll resource.APIResourceGetter[Servo]
}

// NewRPCServiceServer constructs a servo gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Servo], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	servo, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return &pb.MoveResponse{}, servo.Move(ctx, req.GetAngleDeg(), req.Extra.AsMap())
}

func (server *serviceServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	servo, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	angleDeg, err := servo.Position(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{PositionDeg: angleDeg}, nil
}

func (server *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	servo, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, servo.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (server *serviceServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	servo, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := servo.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	servo, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, servo, req)
}
