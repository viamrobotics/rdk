package builtin

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.viam.com/rdk/referenceframe"
)

// replanResponse is the struct returned by the replanner.
type replanResponse struct {
	err    error
	replan bool
}

func (rr replanResponse) String() string {
	if rr.err == nil {
		return fmt.Sprintf("builtin.replanResponse{replan: %t, err: nil}", rr.replan)
	}
	return fmt.Sprintf("builtin.replanResponse{replan: %t, err: %s}", rr.replan, rr.err.Error())
}

// replanFn is an alias for a function that will be polled by a replanner.
type replanFn func(context.Context, [][]referenceframe.Input, int) (bool, error)

// replanner bundles everything needed to execute a function at a given interval and return.
type replanner struct {
	period       time.Duration
	needReplan   replanFn
	responseChan chan replanResponse
}

// newReplanner is a constructor.
func newReplanner(period time.Duration, fnToPoll replanFn) *replanner {
	return &replanner{
		period:       period,
		needReplan:   fnToPoll,
		responseChan: make(chan replanResponse),
	}
}

// startPolling executes the replanner's configured function at its configured period
// The caller of this function should read from the replanner's responseChan to know when a replan is requested.
func (r *replanner) startPolling(ctx context.Context, plan [][]referenceframe.Input, waypointIndex *atomic.Int32) {
	ticker := time.NewTicker(r.period)
	defer ticker.Stop()

	// this check ensures that if the context is cancelled we always return early at the top of the loop
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			index := int(waypointIndex.Load())
			if index >= len(plan) {
				index = len(plan) - 1
			}
			replan, err := r.needReplan(ctx, plan, index)
			if err != nil || replan {
				r.responseChan <- replanResponse{replan: replan, err: err}
				return
			}
		}
	}
}
