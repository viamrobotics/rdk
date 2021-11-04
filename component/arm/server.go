// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/core/proto/api/component/v1"
	oldpb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/subtype"
)

// SubtypeServer implements the contract from arm_subtype.proto
type SubtypeServer struct {
	pb.UnimplementedArmSubtypeServiceServer
	s subtype.Service
}

// New constructs a gRPC service server.
func New(s subtype.Service) pb.ArmSubtypeServiceServer {
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
func (s *SubtypeServer) CurrentPosition(ctx context.Context, req *pb.CurrentPositionRequest) (*pb.CurrentPositionResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	convertedPos := &pb.Position{
		X: pos.X, Y: pos.Y, Z: pos.Z, OX: pos.OX, OY: pos.OY, OZ: pos.OZ, Theta: pos.Theta,
	}
	return &pb.CurrentPositionResponse{Position: convertedPos}, nil
}

// CurrentJointPositions gets the current joint position of an arm of the underlying robot.
func (s *SubtypeServer) CurrentJointPositions(ctx context.Context, req *pb.CurrentJointPositionsRequest) (*pb.CurrentJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	convertedPos := &pb.JointPositions{Degrees: pos.Degrees}
	return &pb.CurrentJointPositionsResponse{Positions: convertedPos}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *SubtypeServer) MoveToPosition(ctx context.Context, req *pb.MoveToPositionRequest) (*pb.MoveToPositionResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	convertedTo := &oldpb.ArmPosition{
		X: req.To.X, Y: req.To.Y, Z: req.To.Z, OX: req.To.OX, OY: req.To.OY, OZ: req.To.OZ, Theta: req.To.Theta,
	}
	return &pb.MoveToPositionResponse{}, arm.MoveToPosition(ctx, convertedTo)
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *SubtypeServer) MoveToJointPositions(ctx context.Context, req *pb.MoveToJointPositionsRequest) (*pb.MoveToJointPositionsResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}
	convertedTo := &oldpb.JointPositions{Degrees: req.To.Degrees}
	return &pb.MoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, convertedTo)
}

// JointMoveDelta moves a specific joint of an arm of the underlying robot by the given amount.
func (s *SubtypeServer) JointMoveDelta(ctx context.Context, req *pb.JointMoveDeltaRequest) (*pb.JointMoveDeltaResponse, error) {
	arm, err := s.getArm(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.JointMoveDeltaResponse{}, arm.JointMoveDelta(ctx, int(req.Joint), req.AmountDegs)
}
