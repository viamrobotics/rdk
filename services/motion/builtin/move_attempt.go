package builtin

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/utils"
)

// moveResponse is a struct that is used to communicate the outcome of a moveAttempt.
type moveResponse struct {
	err     error
	success bool
}

func (mr moveResponse) String() string {
	return fmt.Sprintf("builtin.moveResponse{success: %t, err: %v}", mr.success, mr.err)
}

// moveAttempt is a struct whose lifetime lasts the duration of an attempt to complete a moveRequest
// it contains a context in which the move call executes and tracks the goroutines that it spawns.
type moveAttempt struct {
	ctx               context.Context
	cancelFn          context.CancelFunc
	backgroundWorkers *sync.WaitGroup

	request      *moveRequest
	responseChan chan moveResponse

	// replanners for the move attempt
	// if we ever have to add additional instances we should figure out how to make this more scalable
	position, obstacle *replanner

	// waypointIndex tracks the waypoint we are currently executing on
	waypointIndex *atomic.Int32
}

// newMoveAttempt instantiates a moveAttempt which can later be started.
// The caller of this function is expected to also call the cancel function to clean up after instantiation.
func newMoveAttempt(ctx context.Context, request *moveRequest) *moveAttempt {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	var backgroundWorkers sync.WaitGroup

	var waypointIndex atomic.Int32
	waypointIndex.Store(1)

	// effectively don't poll if the PositionPollingFreqHz is not provided
	positionPollingFreq := time.Duration(math.MaxInt64)
	if request.config.positionPollingFreqHz > 0 {
		positionPollingFreq = time.Duration(1000/request.config.positionPollingFreqHz) * time.Millisecond
	}

	// effectively don't poll if the ObstaclePollingFreqHz is not provided
	obstaclePollingFreq := time.Duration(math.MaxInt64)
	if request.config.obstaclePollingFreqHz > 0 {
		obstaclePollingFreq = time.Duration(1000/request.config.obstaclePollingFreqHz) * time.Millisecond
	}

	return &moveAttempt{
		ctx:               cancelCtx,
		cancelFn:          cancelFn,
		backgroundWorkers: &backgroundWorkers,

		request:      request,
		responseChan: make(chan moveResponse),

		position: newReplanner(positionPollingFreq, request.deviatedFromPlan),
		obstacle: newReplanner(obstaclePollingFreq, request.obstaclesIntersectPlan),

		waypointIndex: &waypointIndex,
	}
}

// start begins a new moveAttempt by using its moveRequest to create a plan, spawn relevant replanners, and finally execute the motion.
// the caller of this function should monitor the moveAttempt's responseChan as well as the replanners' responseChan to get insight
// into the status of the moveAttempt.
func (ma *moveAttempt) start() error {
	waypoints, err := ma.request.plan(ma.ctx)
	if err != nil {
		return err
	}

	ma.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ma.position.startPolling(ma.ctx, waypoints, ma.waypointIndex)
	}, ma.backgroundWorkers.Done)

	ma.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ma.obstacle.startPolling(ma.ctx, waypoints, ma.waypointIndex)
	}, ma.backgroundWorkers.Done)

	// spawn function to execute the plan on the robot
	ma.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		if resp := ma.request.execute(ma.ctx, waypoints, ma.waypointIndex); resp.success || resp.err != nil {
			ma.request.planRequest.Logger.Info("exec resp done")
			ma.responseChan <- resp
		}
		ma.request.planRequest.Logger.Info("exec resp issue")
	}, ma.backgroundWorkers.Done)
	return nil
}

// cancel cleans up a moveAttempt
// it cancels the processes spawned by it, drains all the channels that could have been written to and waits on processes to return.
func (ma *moveAttempt) cancel() {
	ma.request.planRequest.Logger.Info("cancelling")
	ma.cancelFn()
	ma.request.planRequest.Logger.Info("flush pos")
	utils.FlushChan(ma.position.responseChan)
	ma.request.planRequest.Logger.Info("flush obs")
	utils.FlushChan(ma.obstacle.responseChan)
	ma.request.planRequest.Logger.Info("flush resp")
	utils.FlushChan(ma.responseChan)
	ma.request.planRequest.Logger.Info("waiting")
	ma.backgroundWorkers.Wait()
	ma.request.planRequest.Logger.Info("done wait")
}
