// Package gantry contains a gRPC based gantry service server.
package gantry

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gantry/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// subtypeServer implements the GantryService from gantry.proto.
type subtypeServer struct {
	pb.UnimplementedGantryServiceServer
	coll resource.SubtypeCollection[Gantry]
}

// NewRPCServiceServer constructs an gantry gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.SubtypeCollection[Gantry]) interface{} {
	return &subtypeServer{coll: coll}
}

// GetPosition returns the position of the gantry specified.
func (s *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := gantry.Position(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{PositionsMm: pos}, nil
}

// GetLengths gets the lengths of a gantry of the underlying robot.
func (s *subtypeServer) GetLengths(
	ctx context.Context,
	req *pb.GetLengthsRequest,
) (*pb.GetLengthsResponse, error) {
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	lengthsMm, err := gantry.Lengths(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetLengthsResponse{LengthsMm: lengthsMm}, nil
}

// MoveToPosition moves the gantry to the position specified.
func (s *subtypeServer) MoveToPosition(
	ctx context.Context,
	req *pb.MoveToPositionRequest,
) (*pb.MoveToPositionResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToPositionResponse{}, gantry.MoveToPosition(ctx, req.PositionsMm, req.Extra.AsMap())
}

// Stop stops the gantry specified.
func (s *subtypeServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, gantry.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	gantry, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := gantry.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	gantry, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, gantry, req)
}
