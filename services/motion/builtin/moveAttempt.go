package builtin

import (
	"context"
	"sync"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/utils"
)

type moveResponse struct {
	err     error
	success bool
}

type moveAttempt struct {
	ctx               context.Context
	cancelFn          context.CancelFunc
	backgroundWorkers *sync.WaitGroup

	request      *moveRequest
	responseChan chan moveResponse
}

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

func (ma *moveAttempt) cancel() {
	ma.cancelFn()
	utils.FlushChan(ma.request.position.responseChan)
	utils.FlushChan(ma.request.obstacle.responseChan)
	utils.FlushChan(ma.responseChan)
	ma.backgroundWorkers.Wait()
}
