package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	rdkutils "go.viam.com/rdk/utils"
)

// Arm is an injected arm.
type Arm struct {
	arm.Arm
	DoFunc                   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetEndPositionFunc       func(ctx context.Context) (*commonpb.Pose, error)
	MoveToPositionFunc       func(ctx context.Context, to *commonpb.Pose, worldState *commonpb.WorldState) error
	MoveToJointPositionsFunc func(ctx context.Context, pos *pb.JointPositions) error
	GetJointPositionsFunc    func(ctx context.Context) (*pb.JointPositions, error)
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
func (a *Arm) MoveToPosition(ctx context.Context, to *commonpb.Pose, worldState *commonpb.WorldState) error {
	if a.MoveToPositionFunc == nil {
		return a.Arm.MoveToPosition(ctx, to, worldState)
	}
	return a.MoveToPositionFunc(ctx, to, worldState)
}

// MoveToJointPositions calls the injected MoveToJointPositions or the real version.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.Arm.MoveToJointPositions(ctx, jp)
	}
	return a.MoveToJointPositionsFunc(ctx, jp)
}

// GetJointPositions calls the injected GetJointPositions or the real version.
func (a *Arm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
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

// Do calls the injected Do or the real version.
func (a *Arm) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		if doer, ok := a.Arm.(generic.Generic); ok {
			return doer.Do(ctx, cmd)
		}
		return nil, rdkutils.NewUnimplementedInterfaceError("Generic", a.Arm)
	}
	return a.DoFunc(ctx, cmd)
}
