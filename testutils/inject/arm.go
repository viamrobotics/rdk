package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Arm is an injected arm.
type Arm struct {
	arm.Arm
	name                          resource.Name
	DoFunc                        func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	EndPositionFunc               func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)
	MoveToPositionFunc            func(ctx context.Context, to spatialmath.Pose, extra map[string]interface{}) error
	MoveToJointPositionsFunc      func(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error
	MoveThroughJointPositionsFunc func(
		ctx context.Context,
		positions [][]referenceframe.Input,
		options *arm.MoveOptions,
		extra map[string]interface{},
	) error
	JointPositionsFunc func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error)
	StopFunc           func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc       func(context.Context) (bool, error)
	CloseFunc          func(ctx context.Context) error
	KinematicsFunc     func(ctx context.Context) (referenceframe.Model, error)
	CurrentInputsFunc  func(ctx context.Context) ([]referenceframe.Input, error)
	GoToInputsFunc     func(ctx context.Context, inputSteps ...[]referenceframe.Input) error
	GeometriesFunc     func(ctx context.Context) ([]spatialmath.Geometry, error)
	StatusFunc         func(ctx context.Context) (map[string]interface{}, error)
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
		return errtrace.Wrap2(a.Arm.EndPosition(ctx, extra))
	}
	return errtrace.Wrap2(a.EndPositionFunc(ctx, extra))
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Arm) MoveToPosition(ctx context.Context, to spatialmath.Pose, extra map[string]interface{}) error {
	if a.MoveToPositionFunc == nil {
		return errtrace.Wrap(a.Arm.MoveToPosition(ctx, to, extra))
	}
	return errtrace.Wrap(a.MoveToPositionFunc(ctx, to, extra))
}

// MoveToJointPositions calls the injected MoveToJointPositions or the real version.
func (a *Arm) MoveToJointPositions(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error {
	if a.MoveToJointPositionsFunc == nil {
		return errtrace.Wrap(a.Arm.MoveToJointPositions(ctx, positions, extra))
	}
	return errtrace.Wrap(a.MoveToJointPositionsFunc(ctx, positions, extra))
}

// MoveThroughJointPositions calls the injected MoveThroughJointPositions or the real version.
func (a *Arm) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	options *arm.MoveOptions,
	extra map[string]interface{},
) error {
	if a.MoveThroughJointPositionsFunc == nil {
		return errtrace.Wrap(a.Arm.MoveThroughJointPositions(ctx, positions, options, extra))
	}
	return errtrace.Wrap(a.MoveThroughJointPositionsFunc(ctx, positions, options, extra))
}

// JointPositions calls the injected JointPositions or the real version.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
	if a.JointPositionsFunc == nil {
		return errtrace.Wrap2(a.Arm.JointPositions(ctx, extra))
	}
	return errtrace.Wrap2(a.JointPositionsFunc(ctx, extra))
}

// Stop calls the injected Stop or the real version.
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	if a.StopFunc == nil {
		return errtrace.Wrap(a.Arm.Stop(ctx, extra))
	}
	return errtrace.Wrap(a.StopFunc(ctx, extra))
}

// IsMoving calls the injected IsMoving or the real version.
func (a *Arm) IsMoving(ctx context.Context) (bool, error) {
	if a.IsMovingFunc == nil {
		return errtrace.Wrap2(a.Arm.IsMoving(ctx))
	}
	return errtrace.Wrap2(a.IsMovingFunc(ctx))
}

// Close calls the injected Close or the real version.
func (a *Arm) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		if a.Arm == nil {
			return nil
		}
		return errtrace.Wrap(a.Arm.Close(ctx))
	}
	return errtrace.Wrap(a.CloseFunc(ctx))
}

// DoCommand calls the injected DoCommand or the real version.
func (a *Arm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return errtrace.Wrap2(a.Arm.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(a.DoFunc(ctx, cmd))
}

// Kinematics calls the injected Kinematics or the real version.
func (a *Arm) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	if a.KinematicsFunc == nil {
		if a.Arm != nil {
			return errtrace.Wrap2(a.Arm.Kinematics(ctx))
		}
		model := referenceframe.NewSimpleModel("")
		return model, nil
	}
	return errtrace.Wrap2(a.KinematicsFunc(ctx))
}

// CurrentInputs calls the injected CurrentInputs or the real version.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if a.CurrentInputsFunc == nil {
		return errtrace.Wrap2(a.Arm.CurrentInputs(ctx))
	}
	return errtrace.Wrap2(a.CurrentInputsFunc(ctx))
}

// GoToInputs calls the injected GoToInputs or the real version.
func (a *Arm) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	if a.GoToInputsFunc == nil {
		return errtrace.Wrap(a.Arm.GoToInputs(ctx, inputSteps...))
	}
	return errtrace.Wrap(a.GoToInputsFunc(ctx, inputSteps...))
}

// Geometries returns the gripper's geometries.
func (a *Arm) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	if a.GeometriesFunc == nil {
		return errtrace.Wrap2(a.Arm.Geometries(ctx, extra))
	}
	return errtrace.Wrap2(a.GeometriesFunc(ctx))
}

// Status calls the injected Status or the real version.
func (a *Arm) Status(ctx context.Context) (map[string]interface{}, error) {
	if a.StatusFunc != nil {
		return errtrace.Wrap2(a.StatusFunc(ctx))
	}
	if a.Arm != nil {
		return errtrace.Wrap2(a.Arm.Status(ctx))
	}
	return map[string]interface{}{}, nil
}
