package builtin

import (
	"context"
	"sync/atomic"
	"time"

	"go.viam.com/rdk/referenceframe"
)

// replanResponse is the struct returned by the replanner.
type replanResponse struct {
	err    error
	replan bool
}

// replanner bundles everything needed to execute a function at a given interval and return.
type replanner struct {
	period       time.Duration
	fnToPoll     func(context.Context, [][]referenceframe.Input, int) replanResponse
	responseChan chan replanResponse
}

// newReplanner is a constructor.
func newReplanner(period time.Duration, fnToPoll func(context.Context, [][]referenceframe.Input, int) replanResponse) *replanner {
	return &replanner{
		period:       period,
		fnToPoll:     fnToPoll,
		responseChan: make(chan replanResponse),
	}
}

// startPolling executes the replanner's configured function at its configured period
// The caller of this function should read from the replanner's responseChan to know when a replan is requested.
func (r *replanner) startPolling(ctx context.Context, plan [][]referenceframe.Input, waypointIndex *atomic.Int32) {
	ticker := time.NewTicker(r.period)
	defer ticker.Stop()
	for {
		// this ensures that if the context is cancelled we always return early at the top of the loop
		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			response := r.fnToPoll(ctx, plan, int(waypointIndex.Load()))
			if response.err != nil || response.replan {
				r.responseChan <- response
				return
			}
		}
	}
}
