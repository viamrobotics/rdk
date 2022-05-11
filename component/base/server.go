// Package base contains a gRPC based arm service server.
package base

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/operation"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
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
	base, ok := resource.(Base)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a base", name)
	}
	return base, nil
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
	err = base.MoveStraight(ctx, int(req.DistanceMm), mmPerSec)
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
	err = base.MoveArc(ctx, int(req.GetDistanceMm()), mmPerSec, req.GetAngleDeg())
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
	err = base.Spin(ctx, req.GetAngleDeg(), degsPerSec)
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
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}

	err = base.SetPower(ctx, convertVector(req.GetLinear()), convertVector(req.GetAngular()))
	if err != nil {
		return nil, err
	}
	return &pb.SetPowerResponse{}, nil
}

func convertVector(v *commonpb.Vector3) r3.Vector {
	if v == nil {
		return r3.Vector{}
	}
	return r3.Vector{X: v.X, Y: v.Y, Z: v.Z}
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
