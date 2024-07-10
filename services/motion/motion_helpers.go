package motion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// PollHistoryUntilSuccessOrError polls `PlanHistory()` with `req` every `interval`
// until a terminal state is reached.
// An error is returned if the terminal state is Failed, Stopped or an invalid state
// or if the context has an error.
// nil is returned if the terminal state is Succeeded.
func PollHistoryUntilSuccessOrError(
	ctx context.Context,
	m Service,
	interval time.Duration,
	req PlanHistoryReq,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		ph, err := m.PlanHistory(ctx, req)
		if err != nil {
			return err
		}

		status := ph[0].StatusHistory[0]

		switch status.State {
		case PlanStateInProgress:
		case PlanStateFailed:
			err := errors.New("plan failed")
			if reason := status.Reason; reason != nil {
				err = errors.Wrap(err, *reason)
			}
			return err

		case PlanStateStopped:
			return errors.New("plan stopped")

		case PlanStateSucceeded:
			return nil

		default:
			return fmt.Errorf("invalid plan state %d", status.State)
		}

		time.Sleep(interval)
	}
}

// MoveArm is a helper function to abstract away movement for general arms.
func MoveArm(ctx context.Context, logger logging.Logger, a arm.Arm, dst spatialmath.Pose) error {
	inputs, err := a.CurrentInputs(ctx)
	if err != nil {
		return err
	}

	model := a.ModelFrame()
	_, err = model.Transform(inputs)
	if err != nil && strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return errors.New("cannot move arm: " + err.Error())
	} else if err != nil {
		return err
	}

	plan, err := motionplan.PlanFrameMotion(ctx, logger, dst, model, inputs, nil, nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, a, plan)
}
