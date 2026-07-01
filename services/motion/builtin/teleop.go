package builtin

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/utils"
)

const defaultTeleopSmoothAlpha = 0.5

// teleopComponent tracks a single component being teleop'd within the pipeline.
type teleopComponent struct {
	name        string
	moveReqBase motion.MoveReq
	latestPose  atomic.Pointer[referenceframe.PoseInFrame]
}

// emaSmooth applies an exponential moving average to the commanded joint positions:
// out = alpha*target + (1-alpha)*previous, per joint. previous is carried across
// steps and across cycles so the commanded stream stays continuous.
func emaSmooth(target, previous []referenceframe.Input, alpha float64) []referenceframe.Input {
	if previous == nil || len(previous) != len(target) || alpha >= 1.0 {
		result := make([]referenceframe.Input, len(target))
		copy(result, target)
		return result
	}
	b := 1.0 - alpha
	result := make([]referenceframe.Input, len(target))
	for j := range target {
		result[j] = alpha*target[j] + b*previous[j]
	}
	return result
}

// teleopPipeline manages the continuous motion pipeline for low-latency teleop.
// It supports multiple components (arms) planned jointly in a single pipeline
// to guarantee collision-free trajectories.
//
//	notify → Planner goroutine → trajCh → Executor goroutine → arm.GoToInputs()
type teleopPipeline struct {
	logger logging.Logger

	// Immutable after creation.
	cachedFrameSys   *referenceframe.FrameSystem      // built once at pipeline start
	cachedBaseInputs referenceframe.FrameSystemInputs // snapshot at pipeline start

	// Components being teleop'd. Protected by componentsMu.
	componentsMu sync.RWMutex
	components   map[string]*teleopComponent

	// Notification channel — poked when any component gets a new pose.
	// Buffer 1, latest-value semantics.
	notify chan struct{}

	// Trajectory output channel. Buffer 1, one-ahead lookahead.
	trajCh chan motionplan.Trajectory

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

	// EMA-smoothed joint positions per component. Only accessed by executor goroutine.
	smoothedJoints map[string][]referenceframe.Input

	// Lifecycle.
	workers *goutils.StoppableWorkers
}

// trySendNotify pokes the notify channel using latest-value semantics.
// Safe for concurrent callers: never blocks.
func trySendNotify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// runPlanner is the planner goroutine. It wakes on notify signals,
// reads all components' latest poses, and plans a joint trajectory.
func (tp *teleopPipeline) runPlanner(ctx context.Context, ms *builtIn) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tp.notify:
			tp.planOnce(ctx, ms)
		}
	}
}

// planningHeadEqual reports whether two FrameSystemInputs snapshots are identical.
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

func (tp *teleopPipeline) planOnce(ctx context.Context, ms *builtIn) {
	// Snapshot the planning head under RLock: we need a copy both for safe iteration
	// and for the stale-check (planningHeadEqual) after planning completes.
	tp.planningHeadMu.RLock()
	planningHead := make(referenceframe.FrameSystemInputs, len(tp.planningHead))
	for k, v := range tp.planningHead {
		planningHead[k] = v
	}
	tp.planningHeadMu.RUnlock()

	// Build merged inputs from cached base + planning head snapshot.
	inputsStart := time.Now()
	mergedInputs := make(referenceframe.FrameSystemInputs, len(tp.cachedBaseInputs))
	for k, v := range tp.cachedBaseInputs {
		mergedInputs[k] = v
	}
	for k, v := range planningHead {
		mergedInputs[k] = v
	}
	inputsDur := time.Since(inputsStart)
	tp.lastInputsNanos.Store(inputsDur.Nanoseconds())

	// Collect latest poses from all registered components into a multi-frame goal.
	// Planner options are pipeline-level (one joint plan covers every arm), so we take
	// them from a single component chosen deterministically — the lexicographically
	// smallest name — rather than whichever one map iteration happens to yield first.
	// All arms in a pipeline are expected to share planner options.
	tp.componentsMu.RLock()
	goals := make(referenceframe.FrameSystemPoses, len(tp.components))
	var optsComp *teleopComponent
	for _, comp := range tp.components {
		pose := comp.latestPose.Load()
		if pose == nil {
			continue
		}
		goals[comp.name] = pose
		if optsComp == nil || comp.name < optsComp.name {
			optsComp = comp
		}
	}
	var extra map[string]interface{}
	if optsComp != nil {
		extra = tp.buildExtra(optsComp.moveReqBase.Extra, mergedInputs)
	}
	tp.componentsMu.RUnlock()

	if len(goals) == 0 {
		return
	}

	// Plan for all components jointly.
	planStart := time.Now()
	ms.mu.RLock()
	plan, err := ms.planTeleopMulti(ctx, goals, extra, tp.cachedFrameSys, mergedInputs, tp.logger)
	ms.mu.RUnlock()
	planDur := time.Since(planStart)
	tp.lastPlanNanos.Store(planDur.Nanoseconds())
	tp.planCount.Add(1)

	if err != nil {
		tp.lastErr.Store(&err)
		tp.logger.CWarnf(ctx, "teleop planner error (inputs: %s, plan: %s): %v", inputsDur, planDur, err)
		return
	}

	tp.lastErr.Store(nil)
	traj := plan.Trajectory()
	tp.logger.CInfof(ctx, "teleop planner: inputs took: %s, plan took: %s, traj size: %d, components: %d",
		inputsDur, planDur, len(traj), len(goals))
	// A trajectory needs at least a start step plus one motion step; the executor
	// skips step 0, so anything shorter is a no-op. Discard it here rather than
	// enqueue work the executor would silently drop.
	if len(traj) < 2 {
		return
	}

	// Atomically validate the planning head hasn't been reset, update it,
	// and enqueue the trajectory.
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
		// Executor is busy; leave head unchanged and let the next notify trigger a fresh plan.
	}
	tp.planningHeadMu.Unlock()
}

// buildExtra creates the extra map for planner options with teleop defaults and start_state.
func (tp *teleopPipeline) buildExtra(
	baseExtra map[string]interface{},
	startConfig referenceframe.FrameSystemInputs,
) map[string]interface{} {
	extra := make(map[string]interface{}, len(baseExtra)+5)
	for k, v := range baseExtra {
		extra[k] = v
	}

	// Build start_state in the format DeserializePlanState expects.
	confMap := make(map[string]interface{}, len(startConfig))
	for fName, inputs := range startConfig {
		iArr := make([]interface{}, len(inputs))
		for i, v := range inputs {
			iArr[i] = v
		}
		confMap[fName] = iArr
	}
	extra["start_state"] = map[string]interface{}{"configuration": confMap}

	// Apply teleop-optimized planner defaults. These are aggressive because
	// teleop movements are tiny and frequent — we trade solution optimality
	// for speed.
	teleopDefaults := map[string]interface{}{
		"timeout":          5.0,
		"max_ik_solutions": 10,  // fewer solutions = faster (was 20)
		"min_ik_score":     0.1, // accept worse solutions faster (was 0.05)
		"frame_step":       0.1, // fewer trajectory steps (was 0.05)
	}
	for k, v := range teleopDefaults {
		if _, ok := extra[k]; !ok {
			extra[k] = v
		}
	}

	// Clear waypoints — not used in teleop.
	extra["waypoints"] = nil

	return extra
}

// executeTeleop executes a trajectory by sending joint targets to all components
// in parallel. Unlike ms.execute, it skips the step-0 position check (which blocks
// on CurrentInputs gRPC calls) and sends commands to all arms concurrently rather
// than sequentially.
func (tp *teleopPipeline) executeTeleop(ctx context.Context, ms *builtIn, traj motionplan.Trajectory) error {
	if len(traj) < 2 {
		return nil
	}

	// Group inputs per component across all trajectory steps (skip step 0 = start position).
	perComponent := make(map[string][][]referenceframe.Input)
	for i := 1; i < len(traj); i++ {
		for name, inputs := range traj[i] {
			if len(inputs) == 0 {
				continue
			}
			perComponent[name] = append(perComponent[name], inputs)
		}
	}

	// Read teleop config.
	// NOTE: caller (runExecutor) already holds ms.mu.RLock, so safe to read ms.conf directly.
	smoothAlpha := defaultTeleopSmoothAlpha
	interpolateOverride := false
	if ms.conf != nil {
		if ms.conf.TeleopSmoothAlpha > 0 {
			smoothAlpha = ms.conf.TeleopSmoothAlpha
		}
		interpolateOverride = ms.conf.TeleopInterpolateOverride
	}

	// Send joint targets to all components in parallel.
	var wg sync.WaitGroup
	errs := make([]error, len(perComponent))
	idx := 0
	for name, inputs := range perComponent {
		r, ok := ms.components[name]
		if !ok {
			// The planner included this component but it has since left the motion
			// service (e.g. a concurrent reconfigure). Skip it but make the gap visible
			// instead of silently moving only some arms.
			tp.logger.CWarnf(ctx, "teleop executor: component %q in trajectory is not known to the motion service; skipping", name)
			continue
		}
		ie, err := utils.AssertType[framesystem.InputEnabled](r)
		if err != nil {
			tp.logger.CWarnf(ctx, "teleop executor: component %q is not input-enabled (%v); skipping", name, err)
			continue
		}

		// Apply EMA smoothing to joint positions before sending (executor goroutine only).
		smoothed := make([][]referenceframe.Input, len(inputs))
		prev := tp.smoothedJoints[name]
		for k, step := range inputs {
			smoothed[k] = emaSmooth(step, prev, smoothAlpha)
			prev = smoothed[k]
		}
		tp.smoothedJoints[name] = prev

		wg.Add(1)
		go func(i int, ie framesystem.InputEnabled, smoothed [][]referenceframe.Input, r resource.Resource) {
			defer wg.Done()
			var err error
			if armComp, ok := r.(arm.Arm); ok {
				if interpolateOverride {
					err = armComp.MoveThroughJointPositions(ctx, smoothed, nil, map[string]interface{}{
						"waitAtEnd":   true,
						"interpolate": true,
					})
				} else {
					err = armComp.MoveThroughJointPositions(ctx, smoothed, nil, map[string]interface{}{
						"waitAtEnd":   false,
						"interpolate": false,
					})
				}
			} else {
				err = ie.GoToInputs(ctx, smoothed...)
			}
			if err != nil {
				if actuator, ok := r.(inputEnabledActuator); ok {
					//nolint:errcheck
					_ = actuator.Stop(context.WithoutCancel(ctx), nil)
				}
				errs[i] = err
			}
		}(idx, ie, smoothed, r)
		idx++
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return errtrace.Wrap(err)
		}
	}
	return nil
}

// runExecutor is the executor goroutine. It reads trajectories from trajCh
// and executes them in parallel across all components.
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
			ms.mu.RLock()
			err := tp.executeTeleop(ctx, ms, traj)
			ms.mu.RUnlock()
			execDur := time.Since(execStart)
			tp.lastExecNanos.Store(execDur.Nanoseconds())
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
				// Update planning head to smoothed positions (what was actually
				// sent to the arm) so the next plan starts from reality.
				tp.planningHeadMu.Lock()
				for name, joints := range tp.smoothedJoints {
					tp.planningHead[name] = joints
				}
				tp.planningHeadMu.Unlock()
			}
		}
	}
}

// resetPlanningHead sets the planning head to the arm's actual current position
// after an execution error.
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

// stop shuts down the pipeline goroutines.
func (tp *teleopPipeline) stop(ctx context.Context, ms *builtIn) {
	tp.workers.Stop()
}

// addTeleopComponent adds a component to the teleop pipeline, creating the pipeline if needed.
func (ms *builtIn) addTeleopComponent(cmdCtx context.Context, req motion.MoveReq) error {
	ms.teleopMu.Lock()
	defer ms.teleopMu.Unlock()

	ms.mu.RLock()
	// Validate the component.
	if _, ok := ms.components[req.ComponentName]; !ok || req.Destination == nil {
		ms.mu.RUnlock()
		return errtrace.Wrap(fmt.Errorf("component must exist and destination must be set. Component: %v Destination: %v",
			req.ComponentName, req.Destination))
	}

	if ms.teleopPipeline == nil {
		// Create a new pipeline.
		fsInputs, err := ms.fsService.CurrentInputs(cmdCtx)
		if err != nil {
			ms.mu.RUnlock()
			return errtrace.Wrap(err)
		}

		frameSys, err := ms.getFrameSystem(cmdCtx, req.WorldState.Transforms())
		if err != nil {
			ms.mu.RUnlock()
			return errtrace.Wrap(err)
		}
		ms.mu.RUnlock()

		tp := &teleopPipeline{
			logger:           ms.logger.Sublogger("teleop"),
			cachedFrameSys:   frameSys,
			cachedBaseInputs: fsInputs,
			components:       make(map[string]*teleopComponent),
			notify:           make(chan struct{}, 1),
			trajCh:           make(chan motionplan.Trajectory, 1),
			planningHead:     fsInputs,
			smoothedJoints:   make(map[string][]referenceframe.Input),
		}

		comp := &teleopComponent{
			name:        req.ComponentName,
			moveReqBase: req,
		}
		comp.latestPose.Store(req.Destination)
		tp.components[req.ComponentName] = comp

		// Send initial notification to trigger first plan.
		trySendNotify(tp.notify)

		tp.workers = goutils.NewBackgroundStoppableWorkers(
			func(pipelineCtx context.Context) { tp.runPlanner(pipelineCtx, ms) },
			func(pipelineCtx context.Context) { tp.runExecutor(pipelineCtx, ms) },
		)

		ms.teleopPipeline = tp
	} else {
		ms.mu.RUnlock()

		// Add component to existing pipeline.
		tp := ms.teleopPipeline
		tp.componentsMu.Lock()
		comp := &teleopComponent{
			name:        req.ComponentName,
			moveReqBase: req,
		}
		comp.latestPose.Store(req.Destination)
		tp.components[req.ComponentName] = comp
		tp.componentsMu.Unlock()

		// Trigger a replan with the new component included.
		trySendNotify(tp.notify)
	}

	return nil
}

// removeTeleopComponent removes a component from the pipeline.
// If no components remain, the pipeline is stopped.
func (ms *builtIn) removeTeleopComponent(ctx context.Context, componentName string) {
	ms.teleopMu.Lock()
	defer ms.teleopMu.Unlock()

	tp := ms.teleopPipeline
	if tp == nil {
		return
	}

	tp.componentsMu.Lock()
	delete(tp.components, componentName)
	remaining := len(tp.components)
	tp.componentsMu.Unlock()

	if remaining == 0 {
		tp.stop(ctx, ms)
		ms.teleopPipeline = nil
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

	if req, ok := cmd[DoTeleopStart]; ok {
		s, err := utils.AssertType[string](req)
		if err != nil {
			return nil, true, errtrace.Wrap(err)
		}

		var moveReqProto pb.MoveRequest
		if err := protojson.Unmarshal([]byte(s), &moveReqProto); err != nil {
			return nil, true, errtrace.Wrap(err)
		}

		fields := moveReqProto.Extra.AsMap()
		if extra, err := utils.AssertType[map[string]interface{}](fields["fields"]); err == nil {
			v, err := structpb.NewStruct(extra)
			if err != nil {
				return nil, true, errtrace.Wrap(err)
			}
			moveReqProto.Extra = v
		}

		moveReq, err := motion.MoveReqFromProto(&moveReqProto)
		if err != nil {
			return nil, true, errtrace.Wrap(err)
		}

		if err := ms.addTeleopComponent(ctx, moveReq); err != nil {
			return nil, true, errtrace.Wrap(err)
		}

		resp[DoTeleopStart] = true
		return resp, true, nil
	}

	if req, ok := cmd[DoTeleopMove]; ok {
		componentName, _ := cmd["component_name"].(string)

		ms.teleopMu.RLock()
		tp := ms.teleopPipeline
		ms.teleopMu.RUnlock()
		if tp == nil {
			return nil, true, errtrace.Wrap(fmt.Errorf("teleop pipeline is not running; call %s first", DoTeleopStart))
		}

		s, err := utils.AssertType[string](req)
		if err != nil {
			return nil, true, errtrace.Wrap(err)
		}
		var pifProto commonpb.PoseInFrame
		if err := protojson.Unmarshal([]byte(s), &pifProto); err != nil {
			return nil, true, errtrace.Wrap(err)
		}

		pif := referenceframe.ProtobufToPoseInFrame(&pifProto)

		// Update the component's latest pose.
		tp.componentsMu.RLock()
		comp := tp.components[componentName]
		// Backward compat: if no component_name and exactly one component, use it.
		if comp == nil && componentName == "" && len(tp.components) == 1 {
			for _, c := range tp.components {
				comp = c
			}
		}
		registered := make([]string, 0, len(tp.components))
		for name := range tp.components {
			registered = append(registered, name)
		}
		tp.componentsMu.RUnlock()

		if comp == nil {
			if componentName == "" {
				return nil, true, errtrace.Wrap(fmt.Errorf("component_name is required for %s when multiple components are registered; registered: %v",
					DoTeleopMove, registered))
			}
			return nil, true, errtrace.Wrap(fmt.Errorf("component %q not registered in teleop pipeline; registered: %v", componentName, registered))
		}

		if seq, ok := cmd["seq"]; ok {
			if seqF, ok := seq.(float64); ok {
				tp.logger.CDebugf(ctx, "teleop received component=%s seq=%d", comp.name, int64(seqF))
			}
		}

		comp.latestPose.Store(pif)
		trySendNotify(tp.notify)

		resp[DoTeleopMove] = true
		return resp, true, nil
	}

	if _, ok := cmd[DoTeleopStop]; ok {
		componentName, _ := cmd["component_name"].(string)

		if componentName == "" {
			// Backward compat: stop entire pipeline.
			ms.teleopMu.Lock()
			if ms.teleopPipeline != nil {
				ms.teleopPipeline.stop(ctx, ms)
				ms.teleopPipeline = nil
			}
			ms.teleopMu.Unlock()
		} else {
			ms.removeTeleopComponent(ctx, componentName)
		}

		resp[DoTeleopStop] = true
		return resp, true, nil
	}

	if _, ok := cmd[DoTeleopStatus]; ok {
		ms.teleopMu.RLock()
		tp := ms.teleopPipeline
		ms.teleopMu.RUnlock()

		if tp == nil {
			return map[string]any{
				DoTeleopStatus: map[string]any{"running": false},
			}, true, nil
		}

		status := map[string]any{
			"running":           true,
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

		tp.componentsMu.RLock()
		compNames := make([]string, 0, len(tp.components))
		for name := range tp.components {
			compNames = append(compNames, name)
		}
		tp.componentsMu.RUnlock()
		status["components"] = compNames

		resp[DoTeleopStatus] = status
		return resp, true, nil
	}

	return resp, false, nil
}
