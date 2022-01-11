// Package gripper contains a gRPC based gripper service server.
package gripper

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from gripper.proto.
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
func (s *subtypeServer) Open(ctx context.Context, req *pb.GripperServiceOpenRequest) (*pb.GripperServiceOpenResponse, error) {
	gripper, err := s.getGripper(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.GripperServiceOpenResponse{}, gripper.Open(ctx)
}

// Grab requests a gripper of the underlying robot to grab.
func (s *subtypeServer) Grab(ctx context.Context, req *pb.GripperServiceGrabRequest) (*pb.GripperServiceGrabResponse, error) {
	gripper, err := s.getGripper(req.Name)
	if err != nil {
		return nil, err
	}
	grabbed, err := gripper.Grab(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GripperServiceGrabResponse{Grabbed: grabbed}, nil
}
