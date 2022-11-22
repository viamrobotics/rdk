package inject

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/spatialmath"
)

// Arm is an injected arm.
type Arm struct {
	arm.LocalArm
	DoFunc                   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	EndPositionFunc          func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)
	MoveToPositionFunc       func(ctx context.Context, to spatialmath.Pose, ws *commonpb.WorldState, extra map[string]interface{}) error
	MoveToJointPositionsFunc func(ctx context.Context, pos *pb.JointPositions, extra map[string]interface{}) error
	JointPositionsFunc       func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error)
	StopFunc                 func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc             func(context.Context) (bool, error)
	CloseFunc                func(ctx context.Context) error
}

// EndPosition calls the injected EndPosition or the real version.
func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	if a.EndPositionFunc == nil {
		return a.LocalArm.EndPosition(ctx, extra)
	}
	return a.EndPositionFunc(ctx, extra)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Arm) MoveToPosition(
	ctx context.Context,
	to spatialmath.Pose,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	if a.MoveToPositionFunc == nil {
		return a.LocalArm.MoveToPosition(ctx, to, worldState, extra)
	}
	return a.MoveToPositionFunc(ctx, to, worldState, extra)
}

// MoveToJointPositions calls the injected MoveToJointPositions or the real version.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions, extra map[string]interface{}) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.LocalArm.MoveToJointPositions(ctx, jp, extra)
	}
	return a.MoveToJointPositionsFunc(ctx, jp, extra)
}

// JointPositions calls the injected JointPositions or the real version.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	if a.JointPositionsFunc == nil {
		return a.LocalArm.JointPositions(ctx, extra)
	}
	return a.JointPositionsFunc(ctx, extra)
}

// Stop calls the injected Stop or the real version.
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	if a.StopFunc == nil {
		return a.LocalArm.Stop(ctx, extra)
	}
	return a.StopFunc(ctx, extra)
}

// IsMoving calls the injected IsMoving or the real version.
func (a *Arm) IsMoving(ctx context.Context) (bool, error) {
	if a.IsMovingFunc == nil {
		return a.LocalArm.IsMoving(ctx)
	}
	return a.IsMovingFunc(ctx)
}

// Close calls the injected Close or the real version.
func (a *Arm) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		return utils.TryClose(ctx, a.LocalArm)
	}
	return a.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (a *Arm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return a.LocalArm.DoCommand(ctx, cmd)
	}
	return a.DoFunc(ctx, cmd)
}
