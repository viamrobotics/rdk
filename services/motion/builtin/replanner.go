package builtin

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/referenceframe"
)

// replanResponse is the struct returned by the replanner.
type replanResponse struct {
	err    error
	replan bool
}

// replanFn is an alias for a function that will be polled by a replanner.
type replanFn func(context.Context, [][]referenceframe.Input, int) (bool, error)

// replanner bundles everything needed to execute a function at a given interval and return.
type replanner struct {
	id           string
	period       time.Duration
	responseChan chan replanResponse

	// needReplan is a function that returns a bool describing if a replan is needed, as well as an error
	needReplan replanFn
	logger     golog.Logger
}

// newReplanner is a constructor.
func newReplanner(period time.Duration, fnToPoll replanFn, id string, logger golog.Logger) *replanner {
	return &replanner{
		id:           id,
		period:       period,
		needReplan:   fnToPoll,
		responseChan: make(chan replanResponse),
		logger:       logger,
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
			a := time.Now()
			r.logger.Infof("%v FROM REPLANNER - %s: \n", a, r.id)
			replan, err := r.needReplan(ctx, plan, int(waypointIndex.Load()))
			r.logger.Infof("since %v - replanner id %s: \n", time.Since(a), r.id)
			if err != nil || replan {
				r.responseChan <- replanResponse{replan: replan, err: err}
				return
			}
		}
	}
}
