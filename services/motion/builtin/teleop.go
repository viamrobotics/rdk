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

	// Profiling atomics (written by planner/executor, read by status handler).
	lastInputsNanos   atomic.Int64
	lastPlanNanos     atomic.Int64
	lastExecNanos     atomic.Int64
	lastExecWaitNanos atomic.Int64
	planCount         atomic.Int64
	execCount         atomic.Int64

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

// planningHeadEqual reports whether two FrameSystemInputs snapshots are identical.
// Used to detect whether the planning head was reset while a plan was in flight.
func planningHeadEqual(a, b referenceframe.FrameSystemInputs) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || len(av) != len(bv) {
			return false
		}
		for idx := range av {
			if av[idx] != bv[idx] {
				return false
			}
		}
	}
	return true
}

func (tp *teleopPipeline) planOnce(ctx context.Context, ms *builtIn, pose *referenceframe.PoseInFrame) {
	// Read the current planning head for the teleop'd arm.
	tp.planningHeadMu.RLock()
	planningHead := tp.planningHead
	tp.planningHeadMu.RUnlock()

	// Merge live inputs with the planning head: use fresh CurrentInputs for
	// all components (including the other arm in bimanual setups), then overlay
	// the planning head for the teleop'd component so trajectory chaining works.
	// Timing includes RLock acquisition — intentional: wall-clock latency is what
	// matters for diagnosing stutter; lock contention is a real part of that latency.
	inputsStart := time.Now()
	ms.mu.RLock()
	liveInputs, err := ms.fsService.CurrentInputs(ctx)
	ms.mu.RUnlock()
	inputsDur := time.Since(inputsStart)
	tp.lastInputsNanos.Store(inputsDur.Nanoseconds())
	if err != nil {
		tp.lastErr.Store(&err)
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
	planStart := time.Now()
	ms.mu.RLock()
	plan, err := ms.planTeleop(ctx, req, tp.cachedFrameSys, mergedInputs, tp.logger)
	ms.mu.RUnlock()
	planDur := time.Since(planStart)
	tp.lastPlanNanos.Store(planDur.Nanoseconds())
	// Includes failed plans; compare with exec_count for success rate.
	tp.planCount.Add(1)

	if err != nil {
		tp.lastErr.Store(&err)
		tp.logger.CWarnf(ctx, "teleop planner error (inputs: %s, plan: %s): %v", inputsDur, planDur, err)
		return
	}

	tp.lastErr.Store(nil)
	traj := plan.Trajectory()
	tp.logger.CInfof(ctx, "teleop planner: inputs took: %s, plan took: %s, traj size: %d", inputsDur, planDur, len(traj))
	if len(traj) == 0 {
		return
	}

	// Re-acquire the write lock to atomically validate that the planning head
	// hasn't been reset (by an execution error) while we were planning, update
	// it to the last step of this trajectory, and enqueue the trajectory.
	// The send must be non-blocking: a blocking send while holding the lock
	// would deadlock with resetPlanningHead, which also needs the write lock to
	// drain trajCh. If the channel is full the head is left unchanged so the
	// next planning iteration re-plans from the same base.
	lastStep := traj[len(traj)-1]
	tp.planningHeadMu.Lock()
	if !planningHeadEqual(tp.planningHead, planningHead) {
		tp.planningHeadMu.Unlock()
		tp.logger.CDebugf(ctx, "teleop planner: planning head changed during planning, discarding trajectory")
		return
	}
	select {
	case tp.trajCh <- traj:
		tp.planningHead = lastStep
	default:
		// Executor is busy; leave head unchanged and let the next pose trigger a fresh plan.
	}
	tp.planningHeadMu.Unlock()
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
		waitStart := time.Now()
		select {
		case <-ctx.Done():
			return
		case traj := <-tp.trajCh:
			waitDur := time.Since(waitStart)
			tp.lastExecWaitNanos.Store(waitDur.Nanoseconds())

			execStart := time.Now()
			// Skip start-position check (math.MaxFloat64) because the arm
			// is in continuous motion and won't be exactly at the trajectory start.
			ms.mu.RLock()
			err := ms.execute(ctx, traj, math.MaxFloat64)
			ms.mu.RUnlock()
			execDur := time.Since(execStart)
			tp.lastExecNanos.Store(execDur.Nanoseconds())
			// Includes failed executions; compare with plan_count for pipeline health.
			tp.execCount.Add(1)

			if !lastExecEnd.IsZero() {
				cycle := time.Since(lastExecEnd)
				totalCycle += cycle
				moveCount++
				avg := totalCycle / time.Duration(moveCount)
				tp.logger.CInfof(ctx, "teleop executor: wait: %s, execute: %s, cycle: %s, avg cycle: %s (n=%d)",
					waitDur, execDur, cycle, avg, moveCount)
			} else {
				tp.logger.CInfof(ctx, "teleop executor: wait: %s, execute: %s (first move)", waitDur, execDur)
			}
			lastExecEnd = time.Now()

			if err != nil {
				tp.lastErr.Store(&err)
				tp.logger.CWarnf(ctx, "teleop executor error: %v", err)
				tp.resetPlanningHead(ctx, ms)
			} else {
				tp.lastErr.Store(nil)
			}
		}
	}
}

// resetPlanningHead sets the planning head to the arm's actual current position
// after an execution error. Resetting the planning head invalidates all previously
// planned trajectories: any trajectory in trajCh was chained from the old (now
// incorrect) head. The drain of trajCh and the head reset are held under the same
// write lock so that planOnce cannot enqueue a stale trajectory between them.
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
	select {
	case <-tp.trajCh:
	default:
	}
	tp.planningHeadMu.Unlock()
}

// stop shuts down the pipeline goroutines and best-effort stops the arm.
func (tp *teleopPipeline) stop(ctx context.Context, ms *builtIn) {
	tp.workers.Stop()
}

// startTeleopPipeline creates and starts a new teleop pipeline.
func (ms *builtIn) startTeleopPipeline(cmdCtx context.Context, req motion.MoveReq) error {
	// Stop any existing pipeline first.
	ms.teleopMu.Lock()
	if ms.teleopPipeline != nil {
		ms.teleopPipeline.stop(cmdCtx, ms)
	}
	defer ms.teleopMu.Unlock()

	ms.mu.RLock()
	fsInputs, err := ms.fsService.CurrentInputs(cmdCtx)
	if err != nil {
		ms.mu.RUnlock()
		return err
	}

	// Validate the command.
	if _, ok := ms.components[req.ComponentName]; !ok || req.Destination == nil {
		ms.mu.RUnlock()
		return fmt.Errorf("Component must exist and destination must be set. Component: %v Destination: %v",
			req.ComponentName, req.Destination)
	}

	// Build and cache the frame system once for the lifetime of this pipeline.
	// The kinematic structure doesn't change during teleop; Reconfigure() stops
	// the pipeline before any config changes.
	frameSys, err := ms.getFrameSystem(cmdCtx, req.WorldState.Transforms())
	if err != nil {
		ms.mu.RUnlock()
		return err
	}
	ms.mu.RUnlock()

	ms.teleopPipeline = &teleopPipeline{
		logger:         ms.logger.Sublogger("teleop"),
		moveReqBase:    req,
		cachedFrameSys: frameSys,
		poseCh:         make(chan *referenceframe.PoseInFrame, 1),
		trajCh:         make(chan motionplan.Trajectory, 1),
		planningHead:   fsInputs,
	}

	ms.teleopPipeline.poseCh <- req.Destination
	ms.teleopPipeline.workers = goutils.NewBackgroundStoppableWorkers(
		func(pipelineCtx context.Context) { ms.teleopPipeline.runPlanner(pipelineCtx, ms) },
		func(pipelineCtx context.Context) { ms.teleopPipeline.runExecutor(pipelineCtx, ms) },
	)

	return nil
}

// handleTeleopCommand handles teleop DoCommand requests.
// Returns (response, handled, error). If handled is false, the caller should
// continue processing other DoCommand keys.
func (ms *builtIn) handleTeleopCommand(
	ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, bool, error) {
	resp := make(map[string]interface{})

	if req, ok := cmd[DoTeleopStart]; ok {
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
		return resp, true, nil
	}

	if req, ok := cmd[DoTeleopMove]; ok {
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
		if seq, ok := cmd["seq"]; ok {
			if seqF, ok := seq.(float64); ok {
				tp.logger.CDebugf(ctx, "teleop received seq=%d", int64(seqF))
			}
		}
		trySendLatest(tp.poseCh, pif)
		resp[DoTeleopMove] = true
		return resp, true, nil
	}

	if _, ok := cmd[DoTeleopStop]; ok {
		ms.teleopMu.Lock()
		if ms.teleopPipeline != nil {
			ms.teleopPipeline.stop(ctx, ms)
			ms.teleopPipeline = nil
		}
		ms.teleopMu.Unlock()

		resp[DoTeleopStop] = true
		return resp, true, nil
	}

	if _, ok := cmd[DoTeleopStatus]; ok {
		ms.teleopMu.RLock()
		tp := ms.teleopPipeline
		ms.teleopMu.RUnlock()

		if tp == nil {
			return map[string]any{
				"running": tp != nil,
			}, true, nil
		}

		status := map[string]any{
			"queued_poses":      len(tp.poseCh),
			"queued_plans":      len(tp.trajCh),
			"last_inputs_ms":    float64(tp.lastInputsNanos.Load()) / 1e6,
			"last_plan_ms":      float64(tp.lastPlanNanos.Load()) / 1e6,
			"last_exec_ms":      float64(tp.lastExecNanos.Load()) / 1e6,
			"last_exec_wait_ms": float64(tp.lastExecWaitNanos.Load()) / 1e6,
			"plan_count":        tp.planCount.Load(),
			"exec_count":        tp.execCount.Load(),
		}
		if lastErr := tp.lastErr.Load(); lastErr != nil {
			status["error"] = (*lastErr).Error()
		}

		resp[DoTeleopStatus] = status

		return resp, true, nil
	}

	return resp, false, nil
}
