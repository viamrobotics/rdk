package builtin

import (
	"context"
	"time"
)

type replanResponse struct {
	err    error
	replan bool
}

type replanner struct {
	period       time.Duration
	fn           func(ctx context.Context) replanResponse
	responseChan chan replanResponse
}

func (r *replanner) startPolling(ctx context.Context) {
	ticker := time.NewTicker(r.period)
	defer ticker.Stop()
	for {
		// this ensures that if the context is cancelled we always return early at the top of the loop
		if err := ctx.Err(); err != nil {
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
