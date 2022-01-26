package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// Arm is an injected arm.
type Arm struct {
	arm.Arm
	CurrentPositionFunc       func(ctx context.Context) (*commonpb.Pose, error)
	MoveToPositionFunc        func(ctx context.Context, c *commonpb.Pose) error
	MoveToJointPositionsFunc  func(ctx context.Context, pos *pb.ArmJointPositions) error
	CurrentJointPositionsFunc func(ctx context.Context) (*pb.ArmJointPositions, error)
	CloseFunc                 func(ctx context.Context) error
}

// CurrentPosition calls the injected CurrentPosition or the real version.
func (a *Arm) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) {
	if a.CurrentPositionFunc == nil {
		return a.Arm.CurrentPosition(ctx)
	}
	return a.CurrentPositionFunc(ctx)
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

// CurrentJointPositions calls the injected CurrentJointPositions or the real version.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	if a.CurrentJointPositionsFunc == nil {
		return a.Arm.CurrentJointPositions(ctx)
	}
	return a.CurrentJointPositionsFunc(ctx)
}

// Close calls the injected Close or the real version.
func (a *Arm) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		return utils.TryClose(ctx, a.Arm)
	}
	return a.CloseFunc(ctx)
}
