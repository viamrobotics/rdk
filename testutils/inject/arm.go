package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
)

// Arm is an injected arm.
type Arm struct {
	arm.Arm
	GetEndPositionFunc       func(ctx context.Context) (*commonpb.Pose, error)
	MoveToPositionFunc       func(ctx context.Context, c *commonpb.Pose) error
	MoveToJointPositionsFunc func(ctx context.Context, pos *pb.ArmJointPositions) error
	GetJointPositionsFunc    func(ctx context.Context) (*pb.ArmJointPositions, error)
	CloseFunc                func(ctx context.Context) error
}

// GetEndPosition calls the injected GetEndPosition or the real version.
func (a *Arm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	if a.GetEndPositionFunc == nil {
		return a.Arm.GetEndPosition(ctx)
	}
	return a.GetEndPositionFunc(ctx)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Arm) MoveToPosition(ctx context.Context, c *commonpb.Pose) error {
	if a.MoveToPositionFunc == nil {
		return a.Arm.MoveToPosition(ctx, c)
	}
	return a.MoveToPositionFunc(ctx, c)
}

// MoveToJointPositions calls the injected MoveToJointPositions or the real version.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.ArmJointPositions) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.Arm.MoveToJointPositions(ctx, jp)
	}
	return a.MoveToJointPositionsFunc(ctx, jp)
}

// GetJointPositions calls the injected GetJointPositions or the real version.
func (a *Arm) GetJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	if a.GetJointPositionsFunc == nil {
		return a.Arm.GetJointPositions(ctx)
	}
	return a.GetJointPositionsFunc(ctx)
}

// Close calls the injected Close or the real version.
func (a *Arm) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		return utils.TryClose(ctx, a.Arm)
	}
	return a.CloseFunc(ctx)
}
