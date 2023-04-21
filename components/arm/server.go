// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// subtypeServer implements the ArmService from arm.proto.
type subtypeServer struct {
	pb.UnimplementedArmServiceServer
	coll resource.SubtypeCollection[Arm]
}

// NewRPCServiceServer constructs an arm gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.SubtypeCollection[Arm]) interface{} {
	return &subtypeServer{coll: coll}
}

// GetEndPosition returns the position of the arm specified.
func (s *subtypeServer) GetEndPosition(
	ctx context.Context,
	req *pb.GetEndPositionRequest,
) (*pb.GetEndPositionResponse, error) {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.EndPosition(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetEndPositionResponse{Pose: spatialmath.PoseToProtobuf(pos)}, nil
}

// GetJointPositions gets the current joint position of an arm of the underlying robot.
func (s *subtypeServer) GetJointPositions(
	ctx context.Context,
	req *pb.GetJointPositionsRequest,
) (*pb.GetJointPositionsResponse, error) {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.JointPositions(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	convertedPos := &pb.JointPositions{Values: pos.Values}
	return &pb.GetJointPositionsResponse{Positions: convertedPos}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *subtypeServer) MoveToPosition(ctx context.Context, req *pb.MoveToPositionRequest) (*pb.MoveToPositionResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToPositionResponse{}, arm.MoveToPosition(
		ctx,
		spatialmath.NewPoseFromProtobuf(req.GetTo()),
		req.Extra.AsMap(),
	)
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *subtypeServer) MoveToJointPositions(
	ctx context.Context,
	req *pb.MoveToJointPositionsRequest,
) (*pb.MoveToJointPositionsResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.Positions, req.Extra.AsMap())
}

// Stop stops the arm specified.
func (s *subtypeServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, arm.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := arm.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, arm, req)
}
