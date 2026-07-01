package motion

import (
	"context"
	"fmt"
	"time"

	"braces.dev/errtrace"
	"github.com/pkg/errors"
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
			return errtrace.Wrap(err)
		}

		ph, err := m.PlanHistory(ctx, req)
		if err != nil {
			return errtrace.Wrap(err)
		}

		status := ph[0].StatusHistory[0]

		switch status.State {
		case PlanStateInProgress:
		case PlanStateFailed:
			err := errors.New("plan failed")
			if reason := status.Reason; reason != nil {
				err = errors.Wrap(err, *reason)
			}
			return errtrace.Wrap(err)

		case PlanStateStopped:
			return errtrace.Wrap(errors.New("plan stopped"))

		case PlanStateSucceeded:
			return nil

		default:
			return errtrace.Wrap(fmt.Errorf("invalid plan state %d", status.State))
		}

		time.Sleep(interval)
	}
}
