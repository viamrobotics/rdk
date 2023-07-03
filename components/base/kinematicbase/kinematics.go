// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
)

// ErrMovementTimeout is used for when a movement call times out after no movement for some time.
var ErrMovementTimeout = errors.New("movement has timed out")

// KinematicBase is an interface for Bases that also satisfy the ModelFramer and InputEnabled interfaces.
type KinematicBase interface {
	base.Base
	referenceframe.InputEnabled

	Kinematics() referenceframe.Frame
}

// WrapWithKinematics will wrap a Base with the appropriate type of kinematics, allowing it to provide a Frame which can be planned with
// and making it InputEnabled.
func WrapWithKinematics(
	ctx context.Context,
	b base.Base,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	maxLinearVelocityMillisPerSec float64,
	maxAngularVelocityDegsPerSec float64,
) (KinematicBase, error) {
	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	// TP-space PTG planning does not yet support 0 turning radius
	if properties.TurningRadiusMeters == 0 {
		return wrapWithDifferentialDriveKinematics(ctx, b, localizer, limits, maxLinearVelocityMillisPerSec, maxAngularVelocityDegsPerSec)
	}
	return wrapWithPTGKinematics(ctx, b, maxLinearVelocityMillisPerSec, maxAngularVelocityDegsPerSec)
}
