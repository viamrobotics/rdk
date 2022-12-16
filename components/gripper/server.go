// Package gripper contains a gRPC based gripper service server.
package gripper

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/gripper/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the GripperService from gripper.proto.
type subtypeServer struct {
	pb.UnimplementedGripperServiceServer
	s subtype.Service
}

// NewServer constructs an gripper gRPC service server.
func NewServer(s subtype.Service) pb.GripperServiceServer {
	return &subtypeServer{s: s}
}

// getGripper returns the gripper specified, nil if not.
func (s *subtypeServer) getGripper(name string) (Gripper, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no gripper with name (%s)", name)
	}
	gripper, ok := resource.(Gripper)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a gripper", name)
	}
	return gripper, nil
}

// Open opens a gripper of the underlying robot.
func (s *subtypeServer) Open(ctx context.Context, req *pb.OpenRequest) (*pb.OpenResponse, error) {
	gripper, err := s.getGripper(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.OpenResponse{}, gripper.Open(ctx, req.Extra.AsMap())
}

// Grab requests a gripper of the underlying robot to grab.
func (s *subtypeServer) Grab(ctx context.Context, req *pb.GrabRequest) (*pb.GrabResponse, error) {
	gripper, err := s.getGripper(req.Name)
	if err != nil {
		return nil, err
	}
	success, err := gripper.Grab(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GrabResponse{Success: success}, nil
}

// Stop stops the gripper specified.
func (s *subtypeServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gripper, err := s.getGripper(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, gripper.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	gripper, err := s.getGripper(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := gripper.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}
