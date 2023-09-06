package builtin

import (
	"context"
	"time"
)

// replanResponse is the struct returned by the replanner
type replanResponse struct {
	err    error
	replan bool
}

// replanner bundles everything needed to execute a function at a given interval and return
type replanner struct {
	period       time.Duration
	fn           func(ctx context.Context) replanResponse
	responseChan chan replanResponse
}

// startPolling executes the replanner's configured function at its configured period
// The caller of this function should read from the replanner's responseChan to know when a replan is requested
func (r *replanner) startPolling(ctx context.Context) {
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
			response := r.fn(ctx)
			if response.err != nil || response.replan {
				r.responseChan <- response
				return
			}
		}
	}
}
