package builtin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/utils"
)

const (
	teleopStreamRunwayMs     = 30
	teleopStreamIntervalMs   = 10
	teleopStreamLimitMargin  = 0.95
	teleopStreamDiagWindow   = 2 * time.Second
	teleopStreamTargetBuffer = 4
	teleopStreamCloseTimeout = 2 * time.Second
)

var (
	errTeleopStreamClosed   = errors.New("teleop arm stream is closed")
	errTeleopNoStreamLimits = errors.New("arm publishes no joint velocity and acceleration limits; " +
		"set teleop_stream.max_vel_degs_per_sec and teleop_stream.max_acc_degs_per_sec2")
)

// TeleopStreamConfig tunes the arm streams teleop drives. Every field is optional.
type TeleopStreamConfig struct {
	MaxVelDegsPerSec  float64 `json:"max_vel_degs_per_sec"`
	MaxAccDegsPerSec2 float64 `json:"max_acc_degs_per_sec2"`
	ChaseGainPerSec   float64 `json:"chase_gain_per_sec"`
}

func teleopStreamOptions() streaming.StreamOptions {
	runway := int32(teleopStreamRunwayMs)
	interval := int32(teleopStreamIntervalMs)
	window := int32(teleopStreamDiagWindow / time.Second)
	return streaming.NewStreamOptions(motion.TempStreamOptions{
		ArmSideTargetRunwayMs: &runway,
		SendToArmIntervalMs:   &interval,
		DiagnosticsWindowSecs: &window,
	})
}

func configuredLimit(degs float64, dof int) []float64 {
	if degs <= 0 {
		return nil
	}
	rads := make([]float64, dof)
	for i := range rads {
		rads[i] = utils.DegToRad(degs)
	}
	return rads
}

func (c TeleopStreamConfig) jointLimits(armLimits []referenceframe.Limit) (vel, acc []float64, err error) {
	dof := len(armLimits)
	cfgVel := configuredLimit(c.MaxVelDegsPerSec, dof)
	cfgAcc := configuredLimit(c.MaxAccDegsPerSec2, dof)

	armVel, armAcc, ok := referenceframe.TrajectoryLimits(armLimits)
	for i := 0; ok && i < dof; i++ {
		ok = armVel[i] > 0 && armAcc[i] > 0
	}
	if !ok {
		if cfgVel == nil || cfgAcc == nil {
			return nil, nil, errTeleopNoStreamLimits
		}
		return cfgVel, cfgAcc, nil
	}

	vel = make([]float64, dof)
	acc = make([]float64, dof)
	for i := range dof {
		vel[i] = armVel[i] * teleopStreamLimitMargin
		if cfgVel != nil {
			vel[i] = min(vel[i], cfgVel[i])
		}
		acc[i] = armAcc[i] * teleopStreamLimitMargin
		if cfgAcc != nil {
			acc[i] = min(acc[i], cfgAcc[i])
		}
	}
	return vel, acc, nil
}

func (c TeleopStreamConfig) chase() teleopChase {
	tc := teleopChase{gain: defaultTeleopChaseGainPerSec}
	if c.ChaseGainPerSec > 0 {
		tc.gain = min(c.ChaseGainPerSec, teleopChaseMaxGainPerSec)
	}
	return tc
}

func (c TeleopStreamConfig) validate() error {
	if c.ChaseGainPerSec < 0 {
		return fmt.Errorf("teleop_stream.chase_gain_per_sec must be non-negative (0 selects the default), got %v", c.ChaseGainPerSec)
	}
	for name, v := range map[string]float64{"max_vel_degs_per_sec": c.MaxVelDegsPerSec, "max_acc_degs_per_sec2": c.MaxAccDegsPerSec2} {
		if v < 0 {
			return fmt.Errorf("teleop_stream.%s must be non-negative (0 uses the arm's own limit), got %v", name, v)
		}
	}
	return nil
}

type teleopArmStreamRunner func(
	ctx context.Context,
	logger logging.Logger,
	a arm.Arm,
	opts streaming.StreamOptions,
	targets <-chan []referenceframe.Input,
	seed []referenceframe.Input,
	diag *diagnostics.SingleSessionDiagnostics,
) error

type teleopArmStream struct {
	mu      sync.Mutex
	targets chan []referenceframe.Input
	closed  bool

	done chan struct{}
	err  error

	diag   *diagnostics.SingleSessionDiagnostics
	opts   streaming.StreamOptions
	cancel context.CancelFunc
}

func (s *teleopArmStream) isDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *teleopArmStream) push(ctx context.Context, jp []referenceframe.Input) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errTeleopStreamClosed
	}
	select {
	case <-s.done:
		return s.closedErr()
	default:
	}
	select {
	case s.targets <- jp:
		return nil
	case <-s.done:
		return s.closedErr()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *teleopArmStream) closedErr() error {
	if s.err != nil {
		return fmt.Errorf("%w: %w", errTeleopStreamClosed, s.err)
	}
	return errTeleopStreamClosed
}

func (s *teleopArmStream) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.targets)
}

// teleopComponent tracks a single component being teleop'd within the pipeline.
type teleopComponent struct {
	name        string
	moveReqBase motion.MoveReq
	latestPose  atomic.Pointer[referenceframe.PoseInFrame]
	arms        []string
}

type teleopArmState struct {
	name string

	mu       sync.Mutex
	stream   *teleopArmStream
	failed   error
	released bool
}

// teleopPipeline manages the continuous motion pipeline for low-latency teleop.
// It supports multiple components (arms) planned jointly in a single pipeline
// to guarantee collision-free trajectories.
//
//	notify → Planner goroutine → trajCh → Executor goroutine → arm stream
type teleopPipeline struct {
	logger logging.Logger

	// Immutable after creation.
	cachedFrameSys   *referenceframe.FrameSystem      // built once at pipeline start
	cachedBaseInputs referenceframe.FrameSystemInputs // snapshot at pipeline start

	streamOpts   streaming.StreamOptions
	streamConf   TeleopStreamConfig
	runArmStream teleopArmStreamRunner

	streamCtx      context.Context
	streamCancel   context.CancelFunc
	streamWG       sync.WaitGroup
	armsMu         sync.Mutex
	arms           map[string]*teleopArmState
	armUsers       map[string]int
	streamsStopped bool

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

	// Last joint positions commanded per component. Only accessed by the executor goroutine.
	commandedJoints map[string][]referenceframe.Input

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

// executeTeleop sends a trajectory's joint targets to all components: arms through their
// streams, other input-enabled components with GoToInputs. Unlike ms.execute, it skips the
// step-0 position check (which blocks on CurrentInputs gRPC calls).
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

	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for name, steps := range perComponent {
		res, ok := ms.components[name]
		if !ok {
			// The planner included this component but it has since left the motion
			// service (e.g. a concurrent reconfigure). Skip it but make the gap visible
			// instead of silently moving only some arms.
			tp.logger.CWarnf(ctx, "teleop executor: component %q in trajectory is not known to the motion service; skipping", name)
			continue
		}
		ie, ok := res.(framesystem.InputEnabled)
		if !ok {
			tp.logger.CWarnf(ctx, "teleop executor: component %q in trajectory is not input-enabled; skipping", name)
			continue
		}

		if armComp, ok := ie.(arm.Arm); ok {
			sent, err := tp.executeStreamed(ctx, name, armComp, steps)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("teleop stream for %q: %w", name, err))
				mu.Unlock()
			} else if sent {
				tp.commandedJoints[name] = steps[len(steps)-1]
			}
			continue
		}

		tp.commandedJoints[name] = steps[len(steps)-1]
		wg.Add(1)
		go func(name string, ie framesystem.InputEnabled, steps [][]referenceframe.Input) {
			defer wg.Done()
			if err := ie.GoToInputs(ctx, steps...); err != nil {
				if actuator, ok := ie.(inputEnabledActuator); ok {
					//nolint:errcheck
					_ = actuator.Stop(context.WithoutCancel(ctx), nil)
				}
				mu.Lock()
				errs = append(errs, fmt.Errorf("teleop move for %q: %w", name, err))
				mu.Unlock()
			}
		}(name, ie, steps)
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (tp *teleopPipeline) executeStreamed(
	ctx context.Context,
	name string,
	a arm.Arm,
	steps [][]referenceframe.Input,
) (bool, error) {
	comp := tp.armState(name)
	if comp == nil {
		return false, nil
	}

	comp.mu.Lock()
	defer comp.mu.Unlock()
	if comp.released {
		return false, nil
	}
	if comp.failed != nil {
		return false, comp.failed
	}

	s := comp.stream
	if s == nil {
		var err error
		s, err = tp.openArmStream(ctx, a, comp)
		if errors.Is(err, errTeleopNoStreamLimits) {
			comp.failed = err
			tp.logger.CWarnf(ctx, "teleop executor: cannot teleoperate %q: %v", name, err)
			return false, err
		}
		if err != nil {
			return false, fmt.Errorf("opening arm stream: %w", err)
		}
	}

	for _, step := range steps {
		if err := s.push(ctx, step); err != nil {
			if s.isDone() {
				comp.failed = s.closedErr()
				tp.logger.CWarnf(ctx, "teleop executor: arm stream for %q died; %q stays stopped until teleop restarts on it: %v",
					name, name, s.err)
			}
			return false, err
		}
	}
	return true, nil
}

func (tp *teleopPipeline) openArmStream(ctx context.Context, a arm.Arm, comp *teleopArmState) (*teleopArmStream, error) {
	model, err := a.Kinematics(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading arm kinematics for its joint limits: %w", err)
	}
	vel, acc, err := tp.streamConf.jointLimits(model.DoF())
	if err != nil {
		return nil, err
	}
	opts := tp.streamOpts
	opts.MoveOptions.MaxVelRadsJoints = vel
	opts.MoveOptions.MaxAccRadsJoints = acc

	seed, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reading seed joint positions: %w", err)
	}

	streamCtx, cancel := context.WithCancel(tp.streamCtx)
	s := &teleopArmStream{
		targets: make(chan []referenceframe.Input, teleopStreamTargetBuffer),
		done:    make(chan struct{}),
		diag:    diagnostics.New(opts.DiagnosticsWindow),
		opts:    opts,
		cancel:  cancel,
	}
	comp.stream = s

	tp.planningHeadMu.Lock()
	tp.planningHead[comp.name] = seed
	tp.planningHeadMu.Unlock()
	tp.commandedJoints[comp.name] = seed

	name := comp.name
	tp.streamWG.Add(1)
	go func() {
		defer tp.streamWG.Done()
		defer cancel()
		err := tp.runArmStream(streamCtx, tp.logger, a, opts, s.targets, seed, s.diag)
		s.err = err
		close(s.done)
		if err != nil && !errors.Is(err, context.Canceled) {
			tp.logger.CWarnf(streamCtx, "teleop arm stream for %q ended: %v", name, err)
		}
		tp.logger.Infow("teleop arm stream stats", "component", name, "options", opts, "stats", s.diag.Stats())
	}()
	tp.logger.CInfof(ctx, "teleop executor: opened arm stream for %q with joint limits %.0f deg/s, %.0f deg/s^2",
		name, inputsToDeg(vel), inputsToDeg(acc))
	return s, nil
}

func (tp *teleopPipeline) armState(name string) *teleopArmState {
	tp.armsMu.Lock()
	defer tp.armsMu.Unlock()
	if tp.streamsStopped || tp.armUsers[name] == 0 {
		return nil
	}
	st, ok := tp.arms[name]
	if !ok {
		st = &teleopArmState{name: name}
		tp.arms[name] = st
	}
	return st
}

func (tp *teleopPipeline) useArms(names []string) {
	tp.armsMu.Lock()
	defer tp.armsMu.Unlock()
	for _, name := range names {
		tp.armUsers[name]++
	}
}

func (tp *teleopPipeline) releaseArms(names []string) {
	var unused []*teleopArmState
	tp.armsMu.Lock()
	for _, name := range names {
		tp.armUsers[name]--
		if tp.armUsers[name] > 0 {
			continue
		}
		delete(tp.armUsers, name)
		if st, ok := tp.arms[name]; ok {
			delete(tp.arms, name)
			unused = append(unused, st)
		}
	}
	tp.armsMu.Unlock()

	for _, st := range unused {
		st.mu.Lock()
		st.released = true
		s := st.stream
		st.mu.Unlock()
		if s != nil {
			s.close()
		}
	}
}

func (tp *teleopPipeline) armStates() []*teleopArmState {
	tp.armsMu.Lock()
	defer tp.armsMu.Unlock()
	states := make([]*teleopArmState, 0, len(tp.arms))
	for _, st := range tp.arms {
		states = append(states, st)
	}
	return states
}

func inputsToDeg(inputs []referenceframe.Input) []float64 {
	degs := make([]float64, len(inputs))
	for i, v := range inputs {
		degs[i] = utils.RadToDeg(v)
	}
	return degs
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
				if ctx.Err() != nil {
					return
				}
				tp.lastErr.Store(&err)
				tp.logger.CWarnf(ctx, "teleop executor error: %v", err)
				tp.resetPlanningHead(ctx, ms)
			} else {
				tp.lastErr.Store(nil)
				// Start the next plan from what was actually commanded.
				tp.planningHeadMu.Lock()
				for name, joints := range tp.commandedJoints {
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

	clear(tp.commandedJoints)
}

// stop halts the planner and executor, then drains every arm stream.
func (tp *teleopPipeline) stop(ctx context.Context, ms *builtIn) {
	tp.workers.Stop()
	tp.closeStreams(ctx)
}

func (tp *teleopPipeline) closeStreams(ctx context.Context) {
	tp.armsMu.Lock()
	tp.streamsStopped = true
	tp.armsMu.Unlock()
	for _, st := range tp.armStates() {
		st.mu.Lock()
		s := st.stream
		st.mu.Unlock()
		if s != nil {
			s.close()
		}
	}

	streamsDone := make(chan struct{})
	go func() {
		tp.streamWG.Wait()
		close(streamsDone)
	}()
	select {
	case <-streamsDone:
	case <-time.After(teleopStreamCloseTimeout):
		tp.logger.CWarnf(ctx, "teleop arm streams did not drain within %s; cancelling them", teleopStreamCloseTimeout)
	}
	tp.streamCancel()
	select {
	case <-streamsDone:
	case <-time.After(teleopStreamCloseTimeout):
		tp.logger.CErrorf(ctx, "teleop arm streams still running %s after cancel; abandoning them", teleopStreamCloseTimeout)
	}
}

func (tp *teleopPipeline) streamStatus() map[string]any {
	comps := tp.armStates()
	streams := make(map[string]any, len(comps))
	for _, comp := range comps {
		comp.mu.Lock()
		info := map[string]any{
			"open":   comp.stream != nil && !comp.stream.isDone(),
			"failed": comp.failed != nil,
		}
		switch {
		case comp.failed != nil:
			info["failure_reason"] = comp.failed.Error()
		case comp.stream != nil && comp.stream.isDone():
			info["failed"] = true
			info["failure_reason"] = comp.stream.closedErr().Error()
		}
		if s := comp.stream; s != nil {
			info["stats"] = s.diag.Stats()
		}
		comp.mu.Unlock()
		streams[comp.name] = info
	}
	return streams
}

// addTeleopComponent adds a component to the teleop pipeline, creating the pipeline if needed.
func (ms *builtIn) addTeleopComponent(cmdCtx context.Context, req motion.MoveReq) error {
	ms.teleopMu.Lock()
	defer ms.teleopMu.Unlock()

	ms.mu.RLock()
	// Validate the component.
	if _, ok := ms.components[req.ComponentName]; !ok || req.Destination == nil {
		ms.mu.RUnlock()
		return fmt.Errorf("component must exist and destination must be set. Component: %v Destination: %v",
			req.ComponentName, req.Destination)
	}

	if ms.teleopPipeline == nil {
		// Create a new pipeline.
		fsInputs, err := ms.fsService.CurrentInputs(cmdCtx)
		if err != nil {
			ms.mu.RUnlock()
			return err
		}

		frameSys, err := ms.getFrameSystem(cmdCtx, req.WorldState.Transforms())
		if err != nil {
			ms.mu.RUnlock()
			return err
		}

		var streamConf TeleopStreamConfig
		if ms.conf != nil {
			streamConf = ms.conf.TeleopStream
		}
		if streamConf.ChaseGainPerSec > teleopChaseMaxGainPerSec {
			ms.logger.Warnf("teleop_stream.chase_gain_per_sec is %v; using %v, the highest the arm stream stays stable at",
				streamConf.ChaseGainPerSec, teleopChaseMaxGainPerSec)
		}
		streamOpts := teleopStreamOptions()
		runArmStream := ms.teleopRunArmStream
		if runArmStream == nil {
			runArmStream = streamConf.chase().run
		}
		carryingArms := armsCarrying(frameSys, ms.components, req.ComponentName)
		ms.mu.RUnlock()

		streamCtx, streamCancel := context.WithCancel(context.Background())
		tp := &teleopPipeline{
			logger:           ms.logger.Sublogger("teleop"),
			cachedFrameSys:   frameSys,
			cachedBaseInputs: fsInputs,
			streamOpts:       streamOpts,
			streamConf:       streamConf,
			runArmStream:     runArmStream,
			streamCtx:        streamCtx,
			streamCancel:     streamCancel,
			components:       make(map[string]*teleopComponent),
			notify:           make(chan struct{}, 1),
			trajCh:           make(chan motionplan.Trajectory, 1),
			planningHead:     fsInputs,
			commandedJoints:  make(map[string][]referenceframe.Input),
			arms:             make(map[string]*teleopArmState),
			armUsers:         make(map[string]int),
		}
		tp.setComponent(req, carryingArms)

		// Send initial notification to trigger first plan.
		trySendNotify(tp.notify)

		tp.workers = goutils.NewBackgroundStoppableWorkers(
			func(pipelineCtx context.Context) { tp.runPlanner(pipelineCtx, ms) },
			func(pipelineCtx context.Context) { tp.runExecutor(pipelineCtx, ms) },
		)

		ms.teleopPipeline = tp
	} else {
		// Add component to existing pipeline.
		tp := ms.teleopPipeline
		carryingArms := armsCarrying(tp.cachedFrameSys, ms.components, req.ComponentName)
		ms.mu.RUnlock()
		tp.setComponent(req, carryingArms)

		// Trigger a replan with the new component included.
		trySendNotify(tp.notify)
	}

	return nil
}

func (tp *teleopPipeline) setComponent(req motion.MoveReq, arms []string) {
	comp := &teleopComponent{
		name:        req.ComponentName,
		moveReqBase: req,
		arms:        arms,
	}
	comp.latestPose.Store(req.Destination)

	tp.componentsMu.Lock()
	old := tp.components[req.ComponentName]
	tp.components[req.ComponentName] = comp
	tp.componentsMu.Unlock()

	tp.useArms(arms)
	if old != nil {
		tp.releaseArms(old.arms)
	}
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
	comp := tp.components[componentName]
	delete(tp.components, componentName)
	remaining := len(tp.components)
	tp.componentsMu.Unlock()

	if remaining == 0 {
		tp.stop(ctx, ms)
		ms.teleopPipeline = nil
		return
	}
	if comp != nil {
		tp.releaseArms(comp.arms)
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

		if err := ms.addTeleopComponent(ctx, moveReq); err != nil {
			return nil, true, err
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
				return nil, true, fmt.Errorf("component_name is required for %s when multiple components are registered; registered: %v",
					DoTeleopMove, registered)
			}
			return nil, true, fmt.Errorf("component %q not registered in teleop pipeline; registered: %v", componentName, registered)
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
		status["streams"] = tp.streamStatus()

		resp[DoTeleopStatus] = status
		return resp, true, nil
	}

	return resp, false, nil
}
