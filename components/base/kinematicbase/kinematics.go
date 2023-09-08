//go:build !notc

// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// KinematicBase is an interface for Bases that also satisfy the ModelFramer and InputEnabled interfaces.
type KinematicBase interface {
	base.Base
	motion.Localizer
	referenceframe.InputEnabled

	Kinematics() referenceframe.Frame
	// ErrorState takes a complete motionplan, as well as the index of the currently-executing set of inputs, and computes the pose
	// difference between where the robot in fact is, and where it ought to be.
	ErrorState(context.Context, [][]referenceframe.Input, int) (spatialmath.Pose, error)
}

const (
	// LinearVelocityMMPerSec is the linear velocity the base will drive at in mm/s.
	defaultLinearVelocityMMPerSec = 200

	// AngularVelocityMMPerSec is the angular velocity the base will turn with in deg/s.
	defaultAngularVelocityDegsPerSec = 60

	// distThresholdMM is used when the base is moving to a goal. It is considered successful if it is within this radius.
	defaultGoalRadiusMM = 300

	// headingThresholdDegrees is used when the base is moving to a goal.
	// If its heading is within this angle it is considered on the correct path.
	defaultHeadingThresholdDegrees = 8

	// planDeviationThresholdMM is the amount that the base is allowed to deviate from the straight line path it is intended to travel.
	// If it ever exceeds this amount the movement will fail and an error will be returned.
	defaultPlanDeviationThresholdMM = 600.0 // mm

	// timeout is the maximum amount of time that the base is allowed to remain stationary during a movement, else an error is thrown.
	defaultTimeout = time.Second * 10

	// minimumMovementThresholdMM is the amount that a base needs to move for it not to be considered stationary.
	defaultMinimumMovementThresholdMM = 20 // mm

	// maxMoveStraightMM is the maximum distance the base should move with a single MoveStraight command.
	// used to break up large driving segments to prevent error from building up due to slightly incorrect angle.
	defaultMaxMoveStraightMM = 1000

	// maxSpinAngleDeg is the maximum amount of degrees the base should turn with a single Spin command.
	// used to break up large turns into smaller chunks to prevent error from building up.
	defaultMaxSpinAngleDeg = 45

	// positionOnlyMode defines whether motion planning should be done in 2DOF or 3DOF.
	defaultPositionOnlyMode = true
)

// Options contains values used for execution of base movement.
type Options struct {
	// LinearVelocityMMPerSec is the linear velocity the base will drive at in mm/s
	LinearVelocityMMPerSec float64

	// AngularVelocityMMPerSec is the angular velocity the base will turn with in deg/s
	AngularVelocityDegsPerSec float64

	// GoalRadiusMM is used when the base is moving to a goal. It is considered successful if it is within this radius.
	GoalRadiusMM float64

	// HeadingThresholdDegrees is used when the base is moving to a goal.
	// If its heading is within this angle it is considered to be on the correct path.
	HeadingThresholdDegrees float64

	// PlanDeviationThresholdMM is the amount that the base is allowed to deviate from the straight line path it is intended to travel.
	// If it ever exceeds this amount the movement will fail and an error will be returned.
	PlanDeviationThresholdMM float64

	// Timeout is the maximum amount of time that the base is allowed to remain stationary during a movement, else an error is thrown.
	Timeout time.Duration

	// MinimumMovementThresholdMM is the amount that a base needs to move for it not to be considered stationary.
	MinimumMovementThresholdMM float64

	// MaxMoveStraightMM is the maximum distance the base should move with a single MoveStraight command.
	// used to break up large driving segments to prevent error from building up due to slightly incorrect angle.
	MaxMoveStraightMM float64

	// MaxSpinAngleDeg is the maximum amount of degrees the base should turn with a single Spin command.
	// used to break up large turns into smaller chunks to prevent error from building up.
	MaxSpinAngleDeg float64

	// PositionOnlyMode defines whether motion planning should be done in 2DOF or 3DOF.
	// If value is true, planning is done in [x,y]. If value is false, planning is done in [x,y,theta].
	PositionOnlyMode bool
}

// NewKinematicBaseOptions creates a struct with values used for execution of base movement.
// all values are pre-set to reasonable default values and can be changed if desired.
func NewKinematicBaseOptions() Options {
	options := Options{
		LinearVelocityMMPerSec:     defaultLinearVelocityMMPerSec,
		AngularVelocityDegsPerSec:  defaultAngularVelocityDegsPerSec,
		GoalRadiusMM:               defaultGoalRadiusMM,
		HeadingThresholdDegrees:    defaultHeadingThresholdDegrees,
		PlanDeviationThresholdMM:   defaultPlanDeviationThresholdMM,
		Timeout:                    defaultTimeout,
		MinimumMovementThresholdMM: defaultMinimumMovementThresholdMM,
		MaxMoveStraightMM:          defaultMaxMoveStraightMM,
		MaxSpinAngleDeg:            defaultMaxSpinAngleDeg,
		PositionOnlyMode:           defaultPositionOnlyMode,
	}
	return options
}

// WrapWithKinematics will wrap a Base with the appropriate type of kinematics, allowing it to provide a Frame which can be planned with
// and making it InputEnabled.
func WrapWithKinematics(
	ctx context.Context,
	b base.Base,
	logger golog.Logger,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	options Options,
) (KinematicBase, error) {
	if kb, ok := b.(KinematicBase); ok {
		return kb, nil
	}

	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	// TP-space PTG planning does not yet support 0 turning radius
	if properties.TurningRadiusMeters == 0 {
		return wrapWithDifferentialDriveKinematics(ctx, b, logger, localizer, limits, options)
	}
	return wrapWithPTGKinematics(ctx, b, logger, localizer, options)
}
