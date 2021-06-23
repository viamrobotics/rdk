package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
)

// Arm is an injected arm.
type Arm struct {
	arm.Arm
	CurrentPositionFunc       func(ctx context.Context) (*pb.ArmPosition, error)
	MoveToPositionFunc        func(ctx context.Context, c *pb.ArmPosition) error
	MoveToJointPositionsFunc  func(ctx context.Context, pos *pb.JointPositions) error
	CurrentJointPositionsFunc func(ctx context.Context) (*pb.JointPositions, error)
	JointMoveDeltaFunc        func(ctx context.Context, joint int, amount float64) error
	CloseFunc                 func() error
}

// CurrentPosition calls the injected CurrentPosition or the real version.
func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	if a.CurrentPositionFunc == nil {
		return a.Arm.CurrentPosition(ctx)
	}
	return a.CurrentPositionFunc(ctx)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	if a.MoveToPositionFunc == nil {
		return a.Arm.MoveToPosition(ctx, c)
	}
	return a.MoveToPositionFunc(ctx, c)
}

// MoveToJointPositions calls the injected MoveToJointPositions or the real version.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.Arm.MoveToJointPositions(ctx, jp)
	}
	return a.MoveToJointPositionsFunc(ctx, jp)
}

// CurrentJointPositions calls the injected CurrentJointPositions or the real version.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	if a.CurrentJointPositionsFunc == nil {
		return a.Arm.CurrentJointPositions(ctx)
	}
	return a.CurrentJointPositionsFunc(ctx)
}

// JointMoveDelta calls the injected JointMoveDelta or the real version.
func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	if a.JointMoveDeltaFunc == nil {
		return a.Arm.JointMoveDelta(ctx, joint, amountDegs)
	}
	return a.JointMoveDeltaFunc(ctx, joint, amountDegs)
}

// Close calls the injected Close or the real version.
func (a *Arm) Close() error {
	if a.CloseFunc == nil {
		return utils.TryClose(a.Arm)
	}
	return a.CloseFunc()
}
