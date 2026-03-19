package builtin

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/utils"
)

// teleopPipeline manages the continuous motion pipeline for low-latency teleop.
// It runs two goroutines connected by channels:
//
//	poseCh → Planner goroutine → trajCh → Executor goroutine → arm.GoToInputs()
type teleopPipeline struct {
	logger logging.Logger

	// Immutable after creation.
	componentName  string
	moveReqBase    motion.MoveReq
	cachedFrameSys *referenceframe.FrameSystem // built once at pipeline start

	// Channels.
	poseCh chan *referenceframe.PoseInFrame // buffer 1, latest-value semantics
	trajCh chan motionplan.Trajectory       // buffer 1, one-ahead lookahead

	// Planning head: the last configuration the planner planned TO.
	// This allows trajectories to chain seamlessly.
	planningHeadMu sync.RWMutex
	planningHead   referenceframe.FrameSystemInputs

	// Error reporting, pollable via teleop_status.
	lastErr atomic.Pointer[error]

	// Lifecycle.
	workers *goutils.StoppableWorkers
}

// trySendLatest sends pose on ch using latest-value semantics:
// if a stale value is buffered, it is drained first so the new value replaces it.
// Safe for concurrent callers: never blocks.
func trySendLatest(ch chan *referenceframe.PoseInFrame, pose *referenceframe.PoseInFrame) {
	// Fast path: channel is empty, send directly.
	select {
	case ch <- pose:
		return
	default:
	}
	// Channel full — drain stale value and retry.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- pose:
	default:
		// Another writer beat us; their pose is equally fresh.
	}
}

// runPlanner is the planner goroutine. It reads poses from poseCh,
// plans trajectories from the planning head, and sends them on trajCh.
func (tp *teleopPipeline) runPlanner(ctx context.Context, ms *builtIn) {
	for {
		select {
		case <-ctx.Done():
			return
		case pose := <-tp.poseCh:
			tp.planOnce(ctx, ms, pose)
		}
	}
}

func (tp *teleopPipeline) planOnce(ctx context.Context, ms *builtIn, pose *referenceframe.PoseInFrame) {
	// Read the current planning head for the teleop'd arm.
	tp.planningHeadMu.RLock()
	planningHead := tp.planningHead
	tp.planningHeadMu.RUnlock()

	// Merge live inputs with the planning head: use fresh CurrentInputs for
	// all components (including the other arm in bimanual setups), then overlay
	// the planning head for the teleop'd component so trajectory chaining works.
	ms.mu.RLock()
	liveInputs, err := ms.fsService.CurrentInputs(ctx)
	ms.mu.RUnlock()
	if err != nil {
		tp.storeError(err)
		tp.logger.CWarnf(ctx, "teleop planner: failed to get current inputs: %v", err)
		return
	}
	mergedInputs := make(referenceframe.FrameSystemInputs, len(liveInputs))
	for k, v := range liveInputs {
		mergedInputs[k] = v
	}
	// Overlay planning head entries for the teleop'd arm's frames.
	for k, v := range planningHead {
		mergedInputs[k] = v
	}

	// Build a MoveReq with start_state set to the merged config.
	req := tp.buildMoveReq(pose, mergedInputs)

	// Call ms.planTeleop with cached frame system and merged inputs.
	ms.mu.RLock()
	plan, err := ms.planTeleop(ctx, req, tp.cachedFrameSys, mergedInputs, tp.logger)
	ms.mu.RUnlock()

	if err != nil {
		tp.storeError(err)
		tp.logger.CWarnf(ctx, "teleop planner error: %v", err)
		return
	}

	tp.clearError()
	traj := plan.Trajectory()
	tp.logger.Info("Trajectory Size:", len(traj))
	if len(traj) == 0 {
		return
	}

	// Update the planning head to the last step of this trajectory.
	lastStep := traj[len(traj)-1]
	tp.planningHeadMu.Lock()
	tp.planningHead = lastStep
	tp.planningHeadMu.Unlock()

	// Send trajectory to executor. This blocks if the executor is busy,
	// providing natural backpressure.
	select {
	case <-ctx.Done():
		return
	case tp.trajCh <- traj:
	}
}

// buildMoveReq creates a MoveReq from the template with the given destination
// and start_state set to the planning head configuration.
func (tp *teleopPipeline) buildMoveReq(
	pose *referenceframe.PoseInFrame,
	startConfig referenceframe.FrameSystemInputs,
) motion.MoveReq {
	req := tp.moveReqBase
	req.Destination = pose

	// Clone Extra to avoid mutating the template.
	extra := make(map[string]interface{}, len(tp.moveReqBase.Extra)+1)
	for k, v := range tp.moveReqBase.Extra {
		extra[k] = v
	}
	// Build start_state in the format DeserializePlanState expects ([]interface{}
	// values, not native []float64) since this path doesn't go through a proto
	// round-trip that would convert the types.
	confMap := make(map[string]interface{}, len(startConfig))
	for fName, inputs := range startConfig {
		iArr := make([]interface{}, len(inputs))
		for i, v := range inputs {
			iArr[i] = v
		}
		confMap[fName] = iArr
	}
	extra["start_state"] = map[string]interface{}{"configuration": confMap}

	// Apply teleop-optimized planner defaults. These only set values not
	// already present so callers can override via teleop_start extra.
	teleopDefaults := map[string]interface{}{
		"timeout":          5.0,  // seconds; default is 300
		"max_ik_solutions": 20,   // default is 100
		"min_ik_score":     0.05, // default is 0.01
		"frame_step":       0.05, // default is 0.01; reduces trajectory steps from ~14 to ~3-4
	}
	for k, v := range teleopDefaults {
		if _, ok := extra[k]; !ok {
			extra[k] = v
		}
	}

	req.Extra = extra

	return req
}

// runExecutor is the executor goroutine. It reads trajectories from trajCh
// and executes them on the arm via ms.execute.
func (tp *teleopPipeline) runExecutor(ctx context.Context, ms *builtIn) {
	var lastExecEnd time.Time
	var totalCycle time.Duration
	var moveCount int64
	for {
		select {
		case <-ctx.Done():
			return
		case traj := <-tp.trajCh:
			execStart := time.Now()
			// Skip start-position check (math.MaxFloat64) because the arm
			// is in continuous motion and won't be exactly at the trajectory start.
			ms.mu.RLock()
			err := ms.execute(ctx, traj, math.MaxFloat64)
			ms.mu.RUnlock()
			execDur := time.Since(execStart)

			if !lastExecEnd.IsZero() {
				cycle := time.Since(lastExecEnd)
				totalCycle += cycle
				moveCount++
				avg := totalCycle / time.Duration(moveCount)
				tp.logger.CInfof(ctx, "teleop executor: execute took: %s, cycle: %s, avg cycle: %s (n=%d)",
					execDur, cycle, avg, moveCount)
			} else {
				tp.logger.CInfof(ctx, "teleop executor: execute took: %s (first move)", execDur)
			}
			lastExecEnd = time.Now()

			if err != nil {
				tp.storeError(err)
				tp.logger.CWarnf(ctx, "teleop executor error: %v", err)
				tp.resetPlanningHead(ctx, ms)
			} else {
				tp.clearError()
			}
		}
	}
}

// resetPlanningHead sets the planning head to the arm's actual current position.
// Called after execution errors when we don't know where the arm stopped.
func (tp *teleopPipeline) resetPlanningHead(ctx context.Context, ms *builtIn) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	fsInputs, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		tp.logger.CWarnf(ctx, "failed to get current inputs for planning head reset: %v", err)
		return
	}

	tp.planningHeadMu.Lock()
	tp.planningHead = fsInputs
	tp.planningHeadMu.Unlock()
}

// stop shuts down the pipeline goroutines and best-effort stops the arm.
func (tp *teleopPipeline) stop(ctx context.Context, ms *builtIn) {
	tp.workers.Stop()

	// Best-effort stop the arm component.
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if r, ok := ms.components[tp.componentName]; ok {
		if actuator, ok := r.(inputEnabledActuator); ok {
			if err := actuator.Stop(ctx, nil); err != nil {
				tp.logger.CWarnf(ctx, "failed to stop arm on teleop shutdown: %v", err)
			}
		}
	}
}

func (tp *teleopPipeline) storeError(err error) {
	tp.lastErr.Store(&err)
}

func (tp *teleopPipeline) clearError() {
	tp.lastErr.Store(nil)
}

func (tp *teleopPipeline) loadError() error {
	if p := tp.lastErr.Load(); p != nil {
		return *p
	}
	return nil
}

// startTeleopPipeline creates and starts a new teleop pipeline.
func (ms *builtIn) startTeleopPipeline(ctx context.Context, req motion.MoveReq) error {
	// Stop any existing pipeline first (outside locks to avoid deadlock).
	ms.stopTeleopPipeline(ctx)

	ms.mu.RLock()
	fsInputs, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		ms.mu.RUnlock()
		return err
	}

	// Verify the component exists.
	if _, ok := ms.components[req.ComponentName]; !ok {
		ms.mu.RUnlock()
		return fmt.Errorf("component %s not found", req.ComponentName)
	}

	// Build and cache the frame system once for the lifetime of this pipeline.
	// The kinematic structure doesn't change during teleop; Reconfigure() stops
	// the pipeline before any config changes.
	frameSys, err := ms.getFrameSystem(ctx, req.WorldState.Transforms())
	if err != nil {
		ms.mu.RUnlock()
		return err
	}
	ms.mu.RUnlock()

	tp := &teleopPipeline{
		logger:         ms.logger.Sublogger("teleop"),
		componentName:  req.ComponentName,
		moveReqBase:    req,
		cachedFrameSys: frameSys,
		poseCh:         make(chan *referenceframe.PoseInFrame, 1),
		trajCh:         make(chan motionplan.Trajectory, 1),
		planningHead:   fsInputs,
	}

	// If the initial request has a destination, enqueue it.
	if req.Destination != nil {
		tp.poseCh <- req.Destination
	}

	tp.workers = goutils.NewBackgroundStoppableWorkers(
		func(ctx context.Context) { tp.runPlanner(ctx, ms) },
		func(ctx context.Context) { tp.runExecutor(ctx, ms) },
	)

	ms.teleopMu.Lock()
	ms.teleopPipeline = tp
	ms.teleopMu.Unlock()

	return nil
}

// stopTeleopPipeline stops the teleop pipeline if one is running.
// Follows the stop-outside-lock pattern to avoid deadlocks.
func (ms *builtIn) stopTeleopPipeline(ctx context.Context) {
	ms.teleopMu.Lock()
	oldPipeline := ms.teleopPipeline
	ms.teleopPipeline = nil
	ms.teleopMu.Unlock()

	if oldPipeline != nil {
		oldPipeline.stop(ctx, ms)
	}
}

// handleTeleopCommand handles teleop DoCommand requests.
// Returns (response, handled, error). If handled is false, the caller should
// continue processing other DoCommand keys.
func (ms *builtIn) handleTeleopCommand(
	ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, bool, error) {
	resp := make(map[string]interface{})
	handled := false

	if req, ok := cmd[DoTeleopStart]; ok {
		handled = true
		s, err := utils.AssertType[string](req)
		if err != nil {
			return nil, true, err
		}
		var moveReqProto pb.MoveRequest
		if err := protojson.Unmarshal([]byte(s), &moveReqProto); err != nil {
			return nil, true, err
		}
		fields := moveReqProto.Extra.AsMap()
		if extra, err := utils.AssertType[map[string]interface{}](fields["fields"]); err == nil {
			v, err := structpb.NewStruct(extra)
			if err != nil {
				return nil, true, err
			}
			moveReqProto.Extra = v
		}
		moveReq, err := motion.MoveReqFromProto(&moveReqProto)
		if err != nil {
			return nil, true, err
		}
		if err := ms.startTeleopPipeline(ctx, moveReq); err != nil {
			return nil, true, err
		}
		resp[DoTeleopStart] = true
	}

	if req, ok := cmd[DoTeleopMove]; ok {
		handled = true
		ms.teleopMu.RLock()
		tp := ms.teleopPipeline
		ms.teleopMu.RUnlock()
		if tp == nil {
			return nil, true, fmt.Errorf("teleop pipeline is not running; call %s first", DoTeleopStart)
		}

		s, err := utils.AssertType[string](req)
		if err != nil {
			return nil, true, err
		}
		var pifProto commonpb.PoseInFrame
		if err := protojson.Unmarshal([]byte(s), &pifProto); err != nil {
			return nil, true, err
		}
		pif := referenceframe.ProtobufToPoseInFrame(&pifProto)
		trySendLatest(tp.poseCh, pif)
		resp[DoTeleopMove] = true
	}

	if _, ok := cmd[DoTeleopStop]; ok {
		handled = true
		ms.stopTeleopPipeline(ctx)
		resp[DoTeleopStop] = true
	}

	if _, ok := cmd[DoTeleopStatus]; ok {
		handled = true
		ms.teleopMu.RLock()
		tp := ms.teleopPipeline
		ms.teleopMu.RUnlock()

		status := map[string]interface{}{
			"running": tp != nil,
		}
		if tp != nil {
			status["queued_poses"] = len(tp.poseCh)
			status["queued_plans"] = len(tp.trajCh)
			if lastErr := tp.loadError(); lastErr != nil {
				status["error"] = lastErr.Error()
			}
		}
		resp[DoTeleopStatus] = status
	}

	return resp, handled, nil
}
