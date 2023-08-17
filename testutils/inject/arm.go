package inject

import (
	"context"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Arm is an injected arm.
type Arm struct {
	arm.Arm
	name                     resource.Name
	DoFunc                   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	EndPositionFunc          func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)
	MoveToPositionFunc       func(ctx context.Context, to spatialmath.Pose, extra map[string]interface{}) error
	MoveToJointPositionsFunc func(ctx context.Context, pos *pb.JointPositions, extra map[string]interface{}) error
	JointPositionsFunc       func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error)
	StopFunc                 func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc             func(context.Context) (bool, error)
	CloseFunc                func(ctx context.Context) error
	ModelFrameFunc           func() referenceframe.Model
	CurrentInputsFunc        func(ctx context.Context) ([]referenceframe.Input, error)
	GoToInputsFunc           func(ctx context.Context, goal []referenceframe.Input) error
}

// NewArm returns a new injected arm.
func NewArm(name string) *Arm {
	return &Arm{name: arm.Named(name)}
}

// Name returns the name of the resource.
func (a *Arm) Name() resource.Name {
	return a.name
}

// EndPosition calls the injected EndPosition or the real version.
func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	if a.EndPositionFunc == nil {
		return a.Arm.EndPosition(ctx, extra)
	}
	return a.EndPositionFunc(ctx, extra)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Arm) MoveToPosition(ctx context.Context, to spatialmath.Pose, extra map[string]interface{}) error {
	if a.MoveToPositionFunc == nil {
		return a.Arm.MoveToPosition(ctx, to, extra)
	}
	return a.MoveToPositionFunc(ctx, to, extra)
}

// MoveToJointPositions calls the injected MoveToJointPositions or the real version.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions, extra map[string]interface{}) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.Arm.MoveToJointPositions(ctx, jp, extra)
	}
	return a.MoveToJointPositionsFunc(ctx, jp, extra)
}

// JointPositions calls the injected JointPositions or the real version.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	if a.JointPositionsFunc == nil {
		return a.Arm.JointPositions(ctx, extra)
	}
	return a.JointPositionsFunc(ctx, extra)
}

// Stop calls the injected Stop or the real version.
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	if a.StopFunc == nil {
		return a.Arm.Stop(ctx, extra)
	}
	return a.StopFunc(ctx, extra)
}

// IsMoving calls the injected IsMoving or the real version.
func (a *Arm) IsMoving(ctx context.Context) (bool, error) {
	if a.IsMovingFunc == nil {
		return a.Arm.IsMoving(ctx)
	}
	return a.IsMovingFunc(ctx)
}

// Close calls the injected Close or the real version.
func (a *Arm) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		if a.Arm == nil {
			return nil
		}
		return a.Arm.Close(ctx)
	}
	return a.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (a *Arm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return a.Arm.DoCommand(ctx, cmd)
	}
	return a.DoFunc(ctx, cmd)
}

// ModelFrame calls the injected ModelFrame or the real version.
func (a *Arm) ModelFrame() referenceframe.Model {
	if a.ModelFrameFunc == nil {
		if a.Arm != nil {
			return a.Arm.ModelFrame()
		}
		model := referenceframe.NewSimpleModel("")
		return model
	}
	return a.ModelFrameFunc()
}

// CurrentInputs calls the injected CurrentInputs or the real version.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if a.CurrentInputsFunc == nil {
		return a.Arm.CurrentInputs(ctx)
	}
	return a.CurrentInputsFunc(ctx)
}

// GoToInputs calls the injected GoToInputs or the real version.
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	if a.GoToInputsFunc == nil {
		return a.Arm.GoToInputs(ctx, goal)
	}
	return a.GoToInputsFunc(ctx, goal)
}
