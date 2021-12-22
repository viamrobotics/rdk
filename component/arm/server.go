// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"

	"github.com/pkg/errors"

	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/subtype"
)

// subtypeServer implements the contract from arm_subtype.proto
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

// CurrentPosition returns the position of the arm specified.
func (s *subtypeServer) CurrentPosition(
	ctx context.Context,
	req *pb.ArmServiceCurrentPositionRequest,
) (*pb.ArmServiceCurrentPositionResponse, error) {
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
	return &pb.ArmServiceCurrentPositionResponse{Position: convertedPos}, nil
}

// CurrentJointPositions gets the current joint position of an arm of the underlying robot.
func (s *subtypeServer) CurrentJointPositions(
	ctx context.Context,
	req *pb.ArmServiceCurrentJointPositionsRequest,
) (*pb.ArmServiceCurrentJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	convertedPos := &pb.ArmJointPositions{Degrees: pos.Degrees}
	return &pb.ArmServiceCurrentJointPositionsResponse{Positions: convertedPos}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *subtypeServer) MoveToPosition(
	ctx context.Context,
	req *pb.ArmServiceMoveToPositionRequest,
) (*pb.ArmServiceMoveToPositionResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.ArmServiceMoveToPositionResponse{}, arm.MoveToPosition(ctx, req.To)
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *subtypeServer) MoveToJointPositions(
	ctx context.Context,
	req *pb.ArmServiceMoveToJointPositionsRequest,
) (*pb.ArmServiceMoveToJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.ArmServiceMoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.To)
}

// JointMoveDelta moves a specific joint of an arm of the underlying robot by the given amount.
func (s *subtypeServer) JointMoveDelta(
	ctx context.Context,
	req *pb.ArmServiceJointMoveDeltaRequest,
) (*pb.ArmServiceJointMoveDeltaResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.ArmServiceJointMoveDeltaResponse{}, arm.JointMoveDelta(ctx, int(req.Joint), req.AmountDegs)
}
