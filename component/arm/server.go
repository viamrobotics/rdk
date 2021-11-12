// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"

	"github.com/pkg/errors"

	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/subtype"
)

// SubtypeServer implements the contract from arm_subtype.proto
type SubtypeServer struct {
	pb.UnimplementedArmSubtypeServiceServer
	s subtype.Service
}

// NewServer constructs an arm gRPC service server.
func NewServer(s subtype.Service) pb.ArmSubtypeServiceServer {
	return &SubtypeServer{s: s}
}

// getArm returns the arm specified, nil if not.
func (s *SubtypeServer) getArm(name string) (Arm, error) {
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

// CurrentPosition returns the position of the arm specified.
func (s *SubtypeServer) CurrentPosition(ctx context.Context, req *pb.ArmSubtypeServiceCurrentPositionRequest) (*pb.ArmSubtypeServiceCurrentPositionResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	convertedPos := &commonpb.Pose{
		X: pos.X, Y: pos.Y, Z: pos.Z, OX: pos.OX, OY: pos.OY, OZ: pos.OZ, Theta: pos.Theta,
	}
	return &pb.ArmSubtypeServiceCurrentPositionResponse{Position: convertedPos}, nil
}

// CurrentJointPositions gets the current joint position of an arm of the underlying robot.
func (s *SubtypeServer) CurrentJointPositions(ctx context.Context, req *pb.ArmSubtypeServiceCurrentJointPositionsRequest) (*pb.ArmSubtypeServiceCurrentJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	convertedPos := &pb.ArmJointPositions{Degrees: pos.Degrees}
	return &pb.ArmSubtypeServiceCurrentJointPositionsResponse{Positions: convertedPos}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *SubtypeServer) MoveToPosition(ctx context.Context, req *pb.ArmSubtypeServiceMoveToPositionRequest) (*pb.ArmSubtypeServiceMoveToPositionResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.ArmSubtypeServiceMoveToPositionResponse{}, arm.MoveToPosition(ctx, req.To)
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *SubtypeServer) MoveToJointPositions(ctx context.Context, req *pb.ArmSubtypeServiceMoveToJointPositionsRequest) (*pb.ArmSubtypeServiceMoveToJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.ArmSubtypeServiceMoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.To)
}

// JointMoveDelta moves a specific joint of an arm of the underlying robot by the given amount.
func (s *SubtypeServer) JointMoveDelta(ctx context.Context, req *pb.ArmSubtypeServiceJointMoveDeltaRequest) (*pb.ArmSubtypeServiceJointMoveDeltaResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.ArmSubtypeServiceJointMoveDeltaResponse{}, arm.JointMoveDelta(ctx, int(req.Joint), req.AmountDegs)
}
