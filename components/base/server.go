// Package base contains a gRPC based arm service server.
package base

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/base/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// subtypeServer implements the BaseService from base.proto.
type subtypeServer struct {
	pb.UnimplementedBaseServiceServer
	coll resource.SubtypeCollection[Base]
}

// NewRPCServiceServer constructs a base gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.SubtypeCollection[Base]) interface{} {
	return &subtypeServer{coll: coll}
}

// MoveStraight moves a robot's base in a straight line by a given distance, expressed in millimeters
// and a given speed, expressed in millimeters per second.
func (s *subtypeServer) MoveStraight(
	ctx context.Context,
	req *pb.MoveStraightRequest,
) (*pb.MoveStraightResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}

	err = base.MoveStraight(ctx, int(req.GetDistanceMm()), req.GetMmPerSec(), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.MoveStraightResponse{}, nil
}

// Spin spins a robot's base by an given angle, expressed in degrees, and a given
// angular speed, expressed in degrees per second.
func (s *subtypeServer) Spin(
	ctx context.Context,
	req *pb.SpinRequest,
) (*pb.SpinResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}

	err = base.Spin(ctx, req.GetAngleDeg(), req.GetDegsPerSec(), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.SpinResponse{}, nil
}

func (s *subtypeServer) SetPower(
	ctx context.Context,
	req *pb.SetPowerRequest,
) (*pb.SetPowerResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}

	err = base.SetPower(
		ctx,
		protoutils.ConvertVectorProtoToR3(req.GetLinear()),
		protoutils.ConvertVectorProtoToR3(req.GetAngular()),
		req.Extra.AsMap(),
	)
	if err != nil {
		return nil, err
	}
	return &pb.SetPowerResponse{}, nil
}

func (s *subtypeServer) SetVelocity(
	ctx context.Context,
	req *pb.SetVelocityRequest,
) (*pb.SetVelocityResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}

	err = base.SetVelocity(
		ctx,
		protoutils.ConvertVectorProtoToR3(req.GetLinear()),
		protoutils.ConvertVectorProtoToR3(req.GetAngular()),
		req.Extra.AsMap(),
	)
	if err != nil {
		return nil, err
	}
	return &pb.SetVelocityResponse{}, nil
}

// Stop stops a robot's base.
func (s *subtypeServer) Stop(
	ctx context.Context,
	req *pb.StopRequest,
) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	if err = base.Stop(ctx, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, nil
}

// IsMoving queries of a component is in motion.
func (s *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := base.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	base, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, base, req)
}
