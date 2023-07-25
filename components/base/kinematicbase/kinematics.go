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
)

// KinematicBase is an interface for Bases that also satisfy the ModelFramer and InputEnabled interfaces.
type KinematicBase interface {
	base.Base
	referenceframe.InputEnabled

	Kinematics() referenceframe.Frame
}

const (
	// LinearVelocityMillisPerSec is the linear velocity the base will drive at in mm/s.
	defaultLinearVelocityMillisPerSec = 100

	// AngularVelocityMillisPerSec is the angular velocity the base will turn with in deg/s.
	defaultAngularVelocityDegsPerSec = 60

	// distThresholdMM is used when the base is moving to a goal. It is considered successful if it is within this radius.
	defaultDistThresholdMM = 100 // mm

	// headingThresholdDegrees is used when the base is moving to a goal.
	// If its heading is within this angle it is considered on the correct path.
	defaultHeadingThresholdDegrees = 15

	// deviationThreshold is the amount that the base is allowed to deviate from the straight line path it is intended to travel.
	// If it ever exceeds this amount the movement will fail and an error will be returned.
	defaultDeviationThreshold = 600.0 // mm

	// timeout is the maximum amount of time that the base is allowed to remain stationary during a movement, else an error is thrown.
	defaultTimeout = time.Second * 10

	// movementEpsilon is the amount that a base needs to move for it not to be considered stationary.
	defaultmovementEpsilon = 20 // mm
)

// KinematicBaseOptions contains values used for execution of base movement.
type KinematicBaseOptions struct {
	// LinearVelocityMillisPerSec is the linear velocity the base will drive at in mm/s
	LinearVelocityMillisPerSec float64

	// AngularVelocityMillisPerSec is the angular velocity the base will turn with in deg/s
	AngularVelocityDegsPerSec float64

	// DistThresholdMM is used when the base is moving to a goal. It is considered successful if it is within this radius.
	DistThresholdMM float64

	// HeadingThresholdDegrees is used when the base is moving to a goal.
	// If its heading is within this angle it is considered to be on the correct path.
	HeadingThresholdDegrees float64

	// DeviationThreshold is the amount that the base is allowed to deviate from the straight line path it is intended to travel.
	// If it ever exceeds this amount the movement will fail and an error will be returned.
	DeviationThreshold float64

	// Timeout is the maximum amount of time that the base is allowed to remain stationary during a movement, else an error is thrown.
	Timeout time.Duration

	// MovementEpsilon is the amount that a base needs to move for it not to be considered stationary.
	MovementEpsilon float64
}

// NewKinematicBaseOptions creates a struct with values used for execution of base movement.
// all values are pre-set to reasonable default values and can be changed if desired.
func NewKinematicBaseOptions() KinematicBaseOptions {
	options := KinematicBaseOptions{
		LinearVelocityMillisPerSec: defaultLinearVelocityMillisPerSec,
		AngularVelocityDegsPerSec:  defaultAngularVelocityDegsPerSec,
		DistThresholdMM:            defaultDistThresholdMM,
		HeadingThresholdDegrees:    defaultHeadingThresholdDegrees,
		DeviationThreshold:         defaultDeviationThreshold,
		Timeout:                    defaultTimeout,
		MovementEpsilon:            defaultmovementEpsilon,
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
	options KinematicBaseOptions,
) (KinematicBase, error) {
	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	// TP-space PTG planning does not yet support 0 turning radius
	if properties.TurningRadiusMeters == 0 {
		return wrapWithDifferentialDriveKinematics(ctx, b, logger, localizer, limits, options)
	}
	return wrapWithPTGKinematics(ctx, b, options)
}
