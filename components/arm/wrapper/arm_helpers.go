package wrapper

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var defaultArmPlannerOptions = &motionplan.Constraints{
	LinearConstraint: []motionplan.LinearConstraint{},
}

// MoveArm is a helper function to abstract away movement for general arms.
func MoveArm(ctx context.Context, logger logging.Logger, a arm.Arm, dst spatialmath.Pose) error {
	inputs, err := a.CurrentInputs(ctx)
	if err != nil {
		return err
	}

	model, err := a.Kinematics(ctx)
	if err != nil {
		return err
	}
	_, err = model.Transform(inputs)
	if err != nil && strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return errors.New("cannot move arm: " + err.Error())
	} else if err != nil {
		return err
	}

	plan, err := motionplan.PlanFrameMotion(ctx, logger, dst, model, inputs, defaultArmPlannerOptions, nil)
	if err != nil {
		return err
	}
	return a.MoveThroughJointPositions(ctx, plan, nil, nil)
}
