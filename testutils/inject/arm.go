package inject

import (
	"context"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"
)

type Arm struct {
	api.Arm
	CurrentPositionFunc       func(ctx context.Context) (*pb.ArmPosition, error)
	MoveToPositionFunc        func(ctx context.Context, c *pb.ArmPosition) error
	MoveToJointPositionsFunc  func(ctx context.Context, pos *pb.JointPositions) error
	CurrentJointPositionsFunc func(ctx context.Context) (*pb.JointPositions, error)
	JointMoveDeltaFunc        func(ctx context.Context, joint int, amount float64) error
	CloseFunc                 func(ctx context.Context)
}

func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	if a.CurrentPositionFunc == nil {
		return a.Arm.CurrentPosition(ctx)
	}
	return a.CurrentPositionFunc(ctx)
}

func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	if a.MoveToPositionFunc == nil {
		return a.Arm.MoveToPosition(ctx, c)
	}
	return a.MoveToPositionFunc(ctx, c)
}

func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.Arm.MoveToJointPositions(ctx, jp)
	}
	return a.MoveToJointPositionsFunc(ctx, jp)
}

func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	if a.CurrentJointPositionsFunc == nil {
		return a.Arm.CurrentJointPositions(ctx)
	}
	return a.CurrentJointPositionsFunc(ctx)
}

func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	if a.JointMoveDeltaFunc == nil {
		return a.Arm.JointMoveDelta(ctx, joint, amount)
	}
	return a.JointMoveDeltaFunc(ctx, joint, amount)
}

func (a *Arm) Close(ctx context.Context) {
	if a.CloseFunc == nil {
		a.Arm.Close(ctx)
		return
	}
	a.CloseFunc(ctx)
}
