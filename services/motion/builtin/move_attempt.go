package builtin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
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

	return &moveAttempt{
		ctx:               cancelCtx,
		cancelFn:          cancelFn,
		backgroundWorkers: &backgroundWorkers,

		request:      request,
		responseChan: make(chan moveResponse),

		position: newReplanner(time.Duration(1000/request.config.PositionPollingFreqHz)*time.Millisecond, request.deviatedFromPlan),
		obstacle: newReplanner(time.Duration(1000/request.config.ObstaclePollingFreqHz)*time.Millisecond, request.obstaclesIntersectPlan),

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
			ma.responseChan <- resp
		}
	}, ma.backgroundWorkers.Done)
	return nil
}

// cancel cleans up a moveAttempt
// it cancels the processes spawned by it, drains all the channels that could have been written to and waits on processes to return.
func (ma *moveAttempt) cancel() {
	ma.cancelFn()
	utils.FlushChan(ma.position.responseChan)
	utils.FlushChan(ma.obstacle.responseChan)
	utils.FlushChan(ma.responseChan)
	ma.backgroundWorkers.Wait()
}

type exploreAttempt struct {
	ctx               context.Context
	cancelFn          context.CancelFunc
	backgroundWorkers *sync.WaitGroup

	request       *exploreRequest
	responseChan  chan moveResponse
	executionChan chan moveResponse

	frameSystem referenceframe.FrameSystem

	logger golog.Logger
}

// newMoveAttempt instantiates a moveAttempt which can later be started.
// The caller of this function is expected to also call the cancel function to clean up after instantiation.
func newExploreAttempt(ctx context.Context, request *exploreRequest) *exploreAttempt {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	var backgroundWorkers sync.WaitGroup

	return &exploreAttempt{
		ctx:               cancelCtx,
		cancelFn:          cancelFn,
		backgroundWorkers: &backgroundWorkers,
		request:           request,
		responseChan:      make(chan moveResponse),
		executionChan:     make(chan moveResponse),
	}
}

func (ea *exploreAttempt) start() error {
	ea.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ea.checkForObstacles(ea.ctx)
	}, ea.backgroundWorkers.Done)

	// Start executing plan
	ea.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ea.executePlan(ea.ctx)
	}, ea.backgroundWorkers.Done)
	return nil
}

func (ea *exploreAttempt) cancel() {
	ea.cancelFn()
	utils.FlushChan(ea.responseChan)
	utils.FlushChan(ea.executionChan)
	ea.backgroundWorkers.Wait()
}

func (ms *exploreAttempt) checkForObstacles(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentPose := spatialmath.NewZeroPose()

			worldState, err := ms.updateWorldState(ctx)
			if err != nil {
				ms.responseChan <- moveResponse{err: err}
				return
			}

			collisionPose, err := motionplan.CheckPlan(
				ms.request.kinematicBase.Kinematics(),
				ms.request.plan,
				worldState,
				ms.frameSystem,
				currentPose,
				[]referenceframe.Input{{Value: 0}, {Value: 0}},
				spatialmath.NewZeroPose(),
				ms.logger,
			)
			if err != nil {
				if collisionPose.Point().Distance(currentPose.Point()) < validObstacleDistanceMM {
					ms.logger.Debug("collision found")
					ms.responseChan <- moveResponse{success: true, err: err}
					return
				}
				ms.logger.Debug("collision found but outside of range")
				ms.responseChan <- moveResponse{success: false, err: err}
			} else {
				ms.responseChan <- moveResponse{success: false, err: err}
			}
		}
	}
}

func (ms *exploreAttempt) executePlan(ctx context.Context) {
	// background process carry out plan
	for i := 1; i < len(ms.request.plan); i++ {
		if inputEnabledKb, ok := ms.request.kinematicBase.(inputEnabledActuator); ok {
			if err := inputEnabledKb.GoToInputs(ctx, ms.request.plan[i][ms.request.kinematicBase.Name().Name]); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if stopErr := ms.request.kinematicBase.Stop(ctx, nil); stopErr != nil {
					ms.executionChan <- moveResponse{err: err}
				}
				ms.executionChan <- moveResponse{err: err}
			}
		}
	}
	ms.executionChan <- moveResponse{success: true}
}

func (ms *exploreAttempt) updateWorldState(ctx context.Context) (*referenceframe.WorldState, error) {
	detections, err := ms.request.visionService.GetObjectPointClouds(ctx, ms.request.camera.Name().Name, nil)
	if err != nil && strings.Contains(err.Error(), "does not implement a 3D segmenter") {
		ms.logger.Infof("cannot call GetObjectPointClouds on %q as it does not implement a 3D segmenter", ms.request.visionService.Name())
	} else if err != nil {
		return nil, err
	}

	geoms := []spatialmath.Geometry{}
	for i, detection := range detections {
		geometry := detection.Geometry
		label := ms.request.camera.Name().Name + "_transientObstacle_" + strconv.Itoa(i)
		if geometry.Label() != "" {
			label += "_" + geometry.Label()
		}
		geometry.SetLabel(label)
		geoms = append(geoms, geometry)
	}
	// consider having geoms be from the frame of world
	// to accomplish this we need to know the transform from the base to the camera
	gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame((referenceframe.World), geoms)}
	worldState, err := referenceframe.NewWorldState(gifs, nil)
	if err != nil {
		return nil, err
	}
	return worldState, nil
}
