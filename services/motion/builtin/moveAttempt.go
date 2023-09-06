package builtin

import (
	"context"
	"sync"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/utils"
)

// moveResponse is a struct that is used to communicate the outcome of a moveAttempt.
type moveResponse struct {
	err     error
	success bool
}

// moveAttempt is a struct whose lifetime lasts the duration of an attempt to complete a moveRequest
// it contains a context in which the move call executes and tracks the goroutines that it spawns.
type moveAttempt struct {
	ctx               context.Context
	cancelFn          context.CancelFunc
	backgroundWorkers *sync.WaitGroup

	request      *moveRequest
	responseChan chan moveResponse
}

// newMoveAttempt instantiates a moveAttempt which can later be started.
// The caller of this function is expected to also call the cancel function to clean up after instantiation.
func newMoveAttempt(ctx context.Context, request *moveRequest) *moveAttempt {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	var backgroundWorkers sync.WaitGroup

	return &moveAttempt{
		ctx:               cancelCtx,
		cancelFn:          cancelFn,
		backgroundWorkers: &backgroundWorkers,

		request:      request,
		responseChan: make(chan moveResponse),
	}
}

// start begins a new moveAttempt by using its moveRequest to create a plan, spawn relevant replanners, and finally execute the motion.
// the caller of this function should monitor the moveAttempt's responseChan as well as the replanners' responseChan to get insight
// into the status of the moveAttempt.
func (ma *moveAttempt) start() {
	plan, err := ma.request.plan(ma.ctx)
	if err != nil {
		ma.responseChan <- moveResponse{err: err}
	}

	ma.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ma.request.position.startPolling(ma.ctx)
	}, ma.backgroundWorkers.Done)

	ma.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ma.request.obstacle.startPolling(ma.ctx)
	}, ma.backgroundWorkers.Done)

	// spawn function to execute the plan on the robot
	ma.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		if resp := ma.request.execute(ma.ctx, plan); resp.success || resp.err != nil {
			ma.responseChan <- resp
		}
	}, ma.backgroundWorkers.Done)
}

// cancel cleans up a moveAttempt
// it cancels the processes spawned by it, drains all the channels that could have been written to and waits on processes to return
func (ma *moveAttempt) cancel() {
	ma.cancelFn()
	utils.FlushChan(ma.request.position.responseChan)
	utils.FlushChan(ma.request.obstacle.responseChan)
	utils.FlushChan(ma.responseChan)
	ma.backgroundWorkers.Wait()
	close(ma.responseChan)
}
