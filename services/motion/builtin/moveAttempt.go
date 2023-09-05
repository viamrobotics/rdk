package builtin

import (
	"context"
	"sync"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
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
	defer backgroundWorkers.Wait()

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
		ma.responseChan <- ma.request.execute(ma.ctx, plan)
	}, ma.backgroundWorkers.Done)
}

func (ma *moveAttempt) cancel() {
	ma.cancelFn()
	utils.FlushChan(ma.request.position.responseChan)
	utils.FlushChan(ma.request.obstacle.responseChan)
	utils.FlushChan(ma.responseChan)
	ma.backgroundWorkers.Wait()
}

func plan(ctx context.Context, planRequest *motionplan.PlanRequest, kb kinematicbase.KinematicBase) (motionplan.Plan, error) {
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return make(motionplan.Plan, 0), err
	}
	// TODO: this is really hacky and we should figure out a better place to store this information
	if len(kb.Kinematics().DoF()) == 2 {
		inputs = inputs[:2]
	}
	planRequest.StartConfiguration = map[string][]referenceframe.Input{planRequest.Frame.Name(): inputs}

	return motionplan.PlanMotion(ctx, planRequest)
}
