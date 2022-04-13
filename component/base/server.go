// Package base contains a gRPC based arm service server.
package base

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/operation"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the BaseService from base.proto.
type subtypeServer struct {
	pb.UnimplementedBaseServiceServer
	s subtype.Service
}

// NewServer constructs a base gRPC service server.
func NewServer(s subtype.Service) pb.BaseServiceServer {
	return &subtypeServer{s: s}
}

// getBase returns the base specified or nil.
func (s *subtypeServer) getBase(name string) (Base, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no base with name (%s)", name)
	}
	wrap1, err := WrapWithReconfigurable(resource)
	if err != nil {
		return nil, err
	}
	wrap2, ok := wrap1.(Base)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a base", name)
	}
	return wrap2, nil
}

// MoveStraight moves a robot's base in a straight line by a given distance, expressed in millimeters
// and a given speed, expressed in millimeters per second.
func (s *subtypeServer) MoveStraight(
	ctx context.Context,
	req *pb.MoveStraightRequest,
) (*pb.MoveStraightResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}
	mmPerSec := 500.0 // TODO(erh): this is probably the wrong default
	reqMmPerSec := req.GetMmPerSec()
	if reqMmPerSec != 0 {
		mmPerSec = reqMmPerSec
	}
	err = base.MoveStraight(ctx, int(req.DistanceMm), mmPerSec, req.GetBlock())
	if err != nil {
		return nil, err
	}
	return &pb.MoveStraightResponse{}, nil
}

// MoveArc moves the robot's base in an arc by a given distance, expressed in millimeters,
// a given speed, expressed in millimeters per second of movement, and a given angle exoressed in degrees.
func (s *subtypeServer) MoveArc(
	ctx context.Context,
	req *pb.MoveArcRequest,
) (*pb.MoveArcResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}
	mmPerSec := 500.0 // TODO(erh): this is probably the wrong default
	reqMmPerSec := req.GetMmPerSec()
	if reqMmPerSec != 0 {
		mmPerSec = reqMmPerSec
	}
	err = base.MoveArc(ctx, int(req.GetDistanceMm()), mmPerSec, req.GetAngleDeg(), req.GetBlock())
	if err != nil {
		return nil, err
	}
	return &pb.MoveArcResponse{}, nil
}

// Spin spins a robot's base by an given angle, expressed in degrees, and a given
// angular speed, expressed in degrees per second.
func (s *subtypeServer) Spin(
	ctx context.Context,
	req *pb.SpinRequest,
) (*pb.SpinResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}
	degsPerSec := 64.0
	reqDegsPerSec := req.GetDegsPerSec()
	if reqDegsPerSec != 0 {
		degsPerSec = reqDegsPerSec
	}
	err = base.Spin(ctx, req.GetAngleDeg(), degsPerSec, req.GetBlock())
	if err != nil {
		return nil, err
	}
	return &pb.SpinResponse{}, nil
}

// Stop stops a robot's base.
func (s *subtypeServer) Stop(
	ctx context.Context,
	req *pb.StopRequest,
) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}
	if err = base.Stop(ctx); err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, nil
}
