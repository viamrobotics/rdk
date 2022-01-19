// Package base contains a gRPC based arm service server.
package base

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from base.proto.
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
	req *pb.BaseServiceMoveStraightRequest,
) (*pb.BaseServiceMoveStraightResponse, error) {
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
	return &pb.BaseServiceMoveStraightResponse{}, nil
}

// MoveArc moves the robot's base in an arc by a given distance, expressed in millimeters,
// a given speed, expressed in millimeters per second of movement, and a given angle exoressed in degrees.
func (s *subtypeServer) MoveArc(
	ctx context.Context,
	req *pb.BaseServiceMoveArcRequest,
) (*pb.BaseServiceMoveArcResponse, error) {
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
	return &pb.BaseServiceMoveArcResponse{}, nil
}

// Spin spins a robot's base by an given angle, expressed in degrees, and a given
// angular speed, expressed in degrees per second.
func (s *subtypeServer) Spin(
	ctx context.Context,
	req *pb.BaseServiceSpinRequest,
) (*pb.BaseServiceSpinResponse, error) {
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
	return &pb.BaseServiceSpinResponse{}, nil
}

// Stop stops a robot's base.
func (s *subtypeServer) Stop(
	ctx context.Context,
	req *pb.BaseServiceStopRequest,
) (*pb.BaseServiceStopResponse, error) {
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}
	if err = base.Stop(ctx); err != nil {
		return nil, err
	}
	return &pb.BaseServiceStopResponse{}, nil
}

// WidthGet returns the width of a robot's base expressed in millimeters.
func (s *subtypeServer) WidthGet(
	ctx context.Context,
	req *pb.BaseServiceWidthGetRequest,
) (*pb.BaseServiceWidthGetResponse, error) {
	resp := pb.BaseServiceWidthGetResponse{}
	base, err := s.getBase(req.GetName())
	if err != nil {
		return nil, err
	}
	width, err := base.WidthGet(ctx)
	if err != nil {
		return nil, err
	}
	resp.WidthMm = int64(width)
	return &resp, nil
}
