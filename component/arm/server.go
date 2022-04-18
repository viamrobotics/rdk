// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the ArmService from arm.proto.
type subtypeServer struct {
	pb.UnimplementedArmServiceServer
	s subtype.Service
}

// NewServer constructs an arm gRPC service server.
func NewServer(s subtype.Service) pb.ArmServiceServer {
	return &subtypeServer{s: s}
}

// getArm returns the arm specified, nil if not.
func (s *subtypeServer) getArm(name string) (Arm, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no arm with name (%s)", name)
	}
	arm, ok := resource.(Arm)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not an arm", name)
	}
	return arm, nil
}

// GetEndPosition returns the position of the arm specified.
func (s *subtypeServer) GetEndPosition(
	ctx context.Context,
	req *pb.GetEndPositionRequest,
) (*pb.GetEndPositionResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.GetEndPosition(ctx)
	if err != nil {
		return nil, err
	}
	convertedPos := &commonpb.Pose{
		X: pos.X, Y: pos.Y, Z: pos.Z, OX: pos.OX, OY: pos.OY, OZ: pos.OZ, Theta: pos.Theta,
	}
	return &pb.GetEndPositionResponse{Pose: convertedPos}, nil
}

// GetJointPositions gets the current joint position of an arm of the underlying robot.
func (s *subtypeServer) GetJointPositions(
	ctx context.Context,
	req *pb.GetJointPositionsRequest,
) (*pb.GetJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetJointPositionsResponse{JointPositions: pos}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *subtypeServer) MoveToPosition(ctx context.Context, req *pb.MoveToPositionRequest) (*pb.MoveToPositionResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToPositionResponse{}, arm.MoveToPosition(ctx, req.GetTo(), req.GetWorldState())
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *subtypeServer) MoveToJointPositions(
	ctx context.Context,
	req *pb.MoveToJointPositionsRequest,
) (*pb.MoveToJointPositionsResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.JointPositions)
}
