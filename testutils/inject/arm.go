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
	arm.LocalArm
	DoFunc                   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetEndPositionFunc       func(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error)
	MoveToPositionFunc       func(ctx context.Context, to *commonpb.Pose, worldState *commonpb.WorldState, extra map[string]interface{}) error
	MoveToJointPositionsFunc func(ctx context.Context, pos *pb.JointPositions, extra map[string]interface{}) error
	GetJointPositionsFunc    func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error)
	StopFunc                 func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc             func(context.Context) (bool, error)
	CloseFunc                func(ctx context.Context) error
}

// GetEndPosition calls the injected GetEndPosition or the real version.
func (a *Arm) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	if a.GetEndPositionFunc == nil {
		return a.LocalArm.GetEndPosition(ctx, extra)
	}
	return a.GetEndPositionFunc(ctx, extra)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Arm) MoveToPosition(ctx context.Context, to *commonpb.Pose, worldState *commonpb.WorldState, extra map[string]interface{}) error {
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

// GetJointPositions calls the injected GetJointPositions or the real version.
func (a *Arm) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	if a.GetJointPositionsFunc == nil {
		return a.LocalArm.GetJointPositions(ctx, extra)
	}
	return a.GetJointPositionsFunc(ctx, extra)
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

// Do calls the injected Do or the real version.
func (a *Arm) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return a.LocalArm.Do(ctx, cmd)
	}
	return a.DoFunc(ctx, cmd)
}
