package builtin

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/services/motion/builtin/state"
)

// replanResponse is the struct returned by the replanner.
type replanResponse struct {
	err             error
	executeResponse state.ExecuteResponse
}

// replanFn is an alias for a function that will be polled by a replanner.
type replanFn func(context.Context, motionplan.Plan) (state.ExecuteResponse, error)

func (rr replanResponse) String() string {
	return fmt.Sprintf("builtin.replanResponse{executeResponse: %#v, err: %v}", rr.executeResponse, rr.err)
}

// replanner bundles everything needed to execute a function at a given interval and return.
type replanner struct {
	period       time.Duration
	responseChan chan replanResponse

	// needReplan is a function that returns a bool describing if a replan is needed, as well as an error
	needReplan replanFn
}

// newReplanner is a constructor for a replanner.
func newReplanner(period time.Duration, fnToPoll replanFn) *replanner {
	return &replanner{
		period:       period,
		needReplan:   fnToPoll,
		responseChan: make(chan replanResponse, 1),
	}
}

// startPolling executes the replanner's configured function at its configured period
// The caller of this function should read from the replanner's responseChan to know when a replan is requested.
func (r *replanner) startPolling(ctx context.Context, plan motionplan.Plan) {
	ticker := time.NewTicker(r.period)
	defer ticker.Stop()

	// this check ensures that if the context is cancelled we always return early at the top of the loop
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			executeResp, err := r.needReplan(ctx, plan)
			if err != nil || executeResp.Replan {
				res := replanResponse{executeResponse: executeResp, err: err}
				r.responseChan <- res
				return
			}
		}
	}
}
