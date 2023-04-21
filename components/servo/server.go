// Package servo contains a gRPC based servo service server
package servo

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/servo/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type subtypeServer struct {
	pb.UnimplementedServoServiceServer
	coll resource.SubtypeCollection[Servo]
}

// NewRPCServiceServer constructs a servo gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.SubtypeCollection[Servo]) interface{} {
	return &subtypeServer{coll: coll}
}

func (server *subtypeServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	servo, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return &pb.MoveResponse{}, servo.Move(ctx, req.GetAngleDeg(), req.Extra.AsMap())
}

func (server *subtypeServer) GetPosition(
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

func (server *subtypeServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	servo, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, servo.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (server *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
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
func (server *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	servo, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, servo, req)
}
