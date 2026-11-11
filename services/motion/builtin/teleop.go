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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/utils"
)

const defaultTeleopSmoothAlpha = 0.5

// Values for Config.TeleopExecutor.
const (
	// TeleopExecutorStreamed keeps one MoveThroughJointPositionsStreamed stream open per arm for
	// the life of the teleop session and feeds it a trajex-derived trajectory. This is the default.
	TeleopExecutorStreamed = "streamed"
	// TeleopExecutorUnary sends each planned trajectory as one MoveThroughJointPositions call.
	TeleopExecutorUnary = "unary"
)

// Defaults and tunables for the streamed teleop executor.
const (
	// defaultTeleopStreamRunwayMs is how much trajectory the streamed executor keeps buffered
	// inside the arm. Every point committed there is motion the operator can no longer retract,
	// so teleop keeps it far shorter than the 100ms a scripted stream would use.
	defaultTeleopStreamRunwayMs = 30
	// defaultTeleopStreamIntervalMs is the cadence at which the arm's buffer is topped up, and so
	// also the sampling period of the trajectory sent to the arm.
	defaultTeleopStreamIntervalMs = 10
	// defaultTeleopStreamMaxVelDegsPerSec and defaultTeleopStreamMaxAccDegsPerSec2 are the joint
	// limits the trajectory is generated under when the config does not set them.
	defaultTeleopStreamMaxVelDegsPerSec  = 90
	defaultTeleopStreamMaxAccDegsPerSec2 = 360
	// defaultTeleopStreamMaxLeadMs bounds how far the commanded trajectory may run ahead of the
	// arm. While the not-yet-sent trajectory inside trajex is longer than this, newly planned
	// trajectories are skipped (and the planning head left in place), so the next plan heads
	// straight for the operator's latest pose instead of queueing behind stale ones. This is the
	// streamed analogue of the unary executor dropping trajectories while a move is in flight.
	defaultTeleopStreamMaxLeadMs = 100

	// teleopStreamDiagWindow is how much detail each arm stream's diagnostics retain. It only
	// needs to be long enough to always hold the most recent trajex runway reading.
	teleopStreamDiagWindow = 2 * time.Second
	// teleopStreamTargetBuffer is the depth of the channel between the executor and a stream's
	// Run loop. A little slack lets the executor hand over a whole trajectory without waiting on
	// the stream's send to the arm.
	teleopStreamTargetBuffer = 4
	// teleopStreamCloseTimeout bounds how long stop() waits for streams to drain gracefully
	// before cancelling them, and again for the cancelled streams to stop their arms.
	teleopStreamCloseTimeout = 2 * time.Second
	// teleopStreamMaxConsecutiveFails is how many times in a row an arm's stream may die and be
	// reopened before the executor gives up on streaming to that arm and uses the unary path.
	teleopStreamMaxConsecutiveFails = 3
	// teleopStreamHealthyAfter is how long a stream must have run for its failure to reset the
	// consecutive-failure count rather than add to it.
	teleopStreamHealthyAfter = 10 * time.Second
	// teleopStreamMinRetry is the shortest delay before a skipped trajectory triggers a replan.
	teleopStreamMinRetry = 5 * time.Millisecond
)

var (
	// errTeleopStreamClosed is returned by teleopArmStream.push when the stream has been closed
	// or its Run loop has already returned.
	errTeleopStreamClosed = errors.New("teleop arm stream is closed")
	// errTeleopStreamFallback tells executeTeleop that streaming is unavailable for a component
	// and the unary executor should take over for it.
	errTeleopStreamFallback = errors.New("teleop arm stream unavailable, falling back to unary")
)

// TeleopStreamConfig tunes the streamed teleop executor (Config.TeleopExecutor == "streamed").
// Every field is optional; zero selects the default noted on it.
type TeleopStreamConfig struct {
	// ArmSideTargetRunwayMs is how much trajectory to keep buffered inside the arm. Default 30.
	ArmSideTargetRunwayMs int `json:"arm_side_target_runway_ms"`
	// SendToArmIntervalMs is how often the arm's buffer is topped up, and the sampling period of
	// the trajectory sent to it. Must be less than ArmSideTargetRunwayMs. Default 10.
	SendToArmIntervalMs int `json:"send_to_arm_interval_ms"`
	// MaxVelDegsPerSec and MaxAccDegsPerSec2 are the joint limits the trajectory is generated
	// under, applied to every joint. Defaults 90 and 360.
	MaxVelDegsPerSec  float64 `json:"max_vel_degs_per_sec"`
	MaxAccDegsPerSec2 float64 `json:"max_acc_degs_per_sec2"`
	// MaxVelDegsPerSecJoints and MaxAccDegsPerSec2Joints are per-joint limits. When set they take
	// precedence over the scalar limits and must have one entry per joint.
	MaxVelDegsPerSecJoints  []float64 `json:"max_vel_degs_per_sec_joints"`
	MaxAccDegsPerSec2Joints []float64 `json:"max_acc_degs_per_sec2_joints"`
	// MaxTrajexLeadMs is how far the commanded trajectory may run ahead of the arm before newly
	// planned trajectories are skipped. Default 100.
	MaxTrajexLeadMs int `json:"max_trajex_lead_ms"`
	// MaxTargetsDroppedInARow is how many targets in a row trajex may reject before the stream
	// ends. Default 0: the first rejected target ends the stream, and the executor reopens it.
	MaxTargetsDroppedInARow int `json:"max_targets_dropped_in_a_row"`
	// ForceRebaseIfExtendLatencyExceedsMs forces trajex to rebase once adding a target takes
	// longer than this. Default 0: never.
	ForceRebaseIfExtendLatencyExceedsMs int `json:"force_rebase_if_extend_latency_exceeds_ms"`
}

func (c TeleopStreamConfig) streamOptions() streaming.StreamOptions {
	runway := int32(c.ArmSideTargetRunwayMs)
	if runway <= 0 {
		runway = defaultTeleopStreamRunwayMs
	}
	interval := int32(c.SendToArmIntervalMs)
	if interval <= 0 {
		interval = defaultTeleopStreamIntervalMs
	}
	window := int32(teleopStreamDiagWindow / time.Second)
	maxVel := c.MaxVelDegsPerSec
	if maxVel <= 0 {
		maxVel = defaultTeleopStreamMaxVelDegsPerSec
	}
	maxAcc := c.MaxAccDegsPerSec2
	if maxAcc <= 0 {
		maxAcc = defaultTeleopStreamMaxAccDegsPerSec2
	}
	degsToRads := func(degs []float64) []float64 {
		if len(degs) == 0 {
			return nil
		}
		rads := make([]float64, len(degs))
		for i, d := range degs {
			rads[i] = utils.DegToRad(d)
		}
		return rads
	}
	opts := streaming.NewStreamOptions(motion.TempStreamOptions{
		ArmSideTargetRunwayMs: &runway,
		SendToArmIntervalMs:   &interval,
		DiagnosticsWindowSecs: &window,
		MoveOptions: &arm.MoveOptions{
			MaxVelRads:       utils.DegToRad(maxVel),
			MaxAccRads:       utils.DegToRad(maxAcc),
			MaxVelRadsJoints: degsToRads(c.MaxVelDegsPerSecJoints),
			MaxAccRadsJoints: degsToRads(c.MaxAccDegsPerSec2Joints),
		},
	})
	opts.MaxTargetsDroppedInARowDueToTrajexExtendError = c.MaxTargetsDroppedInARow
	opts.ForceRebaseIfExtendLatencyExceedsMs = c.ForceRebaseIfExtendLatencyExceedsMs
	return opts
}

func (c TeleopStreamConfig) maxLead() time.Duration {
	if c.MaxTrajexLeadMs <= 0 {
		return defaultTeleopStreamMaxLeadMs * time.Millisecond
	}
	return time.Duration(c.MaxTrajexLeadMs) * time.Millisecond
}

func (c TeleopStreamConfig) validate() error {
	if c.MaxTrajexLeadMs < 0 {
		return fmt.Errorf("teleop_stream.max_trajex_lead_ms must be non-negative (0 selects the default), got %d", c.MaxTrajexLeadMs)
	}
	opts := c.streamOptions()
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("teleop_stream: %w", err)
	}
	return nil
}

// teleopArmStreamRunner is the signature of streaming.Run, the function that owns one arm's
// MoveThroughJointPositionsStreamed stream and the trajex session feeding it.
type teleopArmStreamRunner func(
	ctx context.Context,
	logger logging.Logger,
	a arm.Arm,
	opts streaming.StreamOptions,
	targets <-chan []referenceframe.Input,
	seed []referenceframe.Input,
	diag *diagnostics.SingleSessionDiagnostics,
	acks chan<- motion.TempStreamResponse,
) error

// teleopArmStream is one arm's open stream: the target channel feeding its Run loop plus the
// bookkeeping the executor needs to decide whether to push to it, skip, or reopen it.
type teleopArmStream struct {
	// mu serializes push against close so a target is never sent on a closed channel.
	mu      sync.Mutex
	targets chan []referenceframe.Input
	closed  bool

	// done is closed once Run has returned; err is valid from then on.
	done chan struct{}
	err  error

	diag    *diagnostics.SingleSessionDiagnostics
	cancel  context.CancelFunc
	started time.Time
	pushed  atomic.Int64
}

func (s *teleopArmStream) isDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// push hands one joint target to the stream. It may block briefly while the Run loop is busy
// sending to the arm (bounded by the send interval). It returns errTeleopStreamClosed, wrapping
// Run's error if any, once the stream is closed or Run has returned.
func (s *teleopArmStream) push(ctx context.Context, jp []referenceframe.Input) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errTeleopStreamClosed
	}
	// Check done first: a dead Run never drains targets, and a buffered send would otherwise
	// succeed silently while the arm receives nothing.
	select {
	case <-s.done:
		return s.closedErr()
	default:
	}
	select {
	case s.targets <- jp:
		s.pushed.Add(1)
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

// close ends the stream gracefully: Run sends trajex's remaining trajectory to the arm, waits
// for the arm to finish executing it, and returns.
func (s *teleopArmStream) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.targets)
}

// trajexRunway reports how much trajectory the stream's trajex session holds that has not yet
// been sent to the arm, as of Run's most recent reading (it records one on every target and
// every send-interval tick). Run's acknowledgments report the same number but only when a target
// is added, so a value read from them would go stale, and never recover, while trajectories are
// being skipped.
func (s *teleopArmStream) trajexRunway() time.Duration {
	details := s.diag.LastWindowDetails()
	if n := len(details.TrajexRunway); n > 0 {
		return time.Duration(details.TrajexRunway[n-1].SizeMs * float64(time.Millisecond))
	}
	return 0
}

// teleopComponent tracks a single component being teleop'd within the pipeline.
type teleopComponent struct {
	name        string
	moveReqBase motion.MoveReq
	latestPose  atomic.Pointer[referenceframe.PoseInFrame]
}

// teleopArmState is the streamed executor's state for one arm. It is keyed by the arm's name,
// which is not the teleop component's name when the component is a frame the arm carries (a
// gripper, say). The executor goroutine is the only one that opens streams and pushes to them;
// stop, removal, and the status handler read and close.
type teleopArmState struct {
	name string

	mu                   sync.Mutex
	stream               *teleopArmStream
	streamFails          int   // consecutive stream failures
	streamRestarts       int64 // total times the stream has died
	streamSkipped        int64 // trajectories skipped because the stream was too far ahead of the arm
	streamFallbackUnary  bool  // streaming has been given up for this arm
	streamFallbackReason string
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
//	notify → Planner goroutine → trajCh → Executor goroutine → arm stream (or arm.MoveThroughJointPositions)
//
// With the streamed executor, each arm has one MoveThroughJointPositionsStreamed stream open for
// the life of the session, run by streaming.Run. The executor pushes every planned step into the
// arm's stream as a joint target; Run turns the chain of targets into a velocity- and
// acceleration-limited trajectory and keeps a short runway of it buffered in the arm.
type teleopPipeline struct {
	logger logging.Logger

	// Immutable after creation.
	cachedFrameSys   *referenceframe.FrameSystem      // built once at pipeline start
	cachedBaseInputs referenceframe.FrameSystemInputs // snapshot at pipeline start

	// Streamed executor configuration. Immutable after creation.
	streamed     bool
	streamOpts   streaming.StreamOptions
	maxLead      time.Duration
	runArmStream teleopArmStreamRunner

	// Arm streams outlive the worker goroutines so stop() can drain them gracefully after the
	// executor has halted. streamCancel is the hard stop (Run then stops the arm); streamWG
	// tracks the Run goroutines.
	streamCtx    context.Context
	streamCancel context.CancelFunc
	streamWG     sync.WaitGroup
	// retryPending dedups the replan timer armed when a trajectory is skipped.
	retryPending atomic.Bool
	// Per-arm streamed-executor state, created the first time a trajectory moves the arm.
	// streamsStopped is set once the pipeline is stopping; no stream may open after that.
	armsMu         sync.Mutex
	arms           map[string]*teleopArmState
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

	// Last joint positions actually commanded per component: the EMA output on the unary path,
	// the last target pushed on the streamed path. Only accessed by the executor goroutine.
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

// unaryJob is one component's share of a trajectory, destined for the unary executor.
type unaryJob struct {
	name  string
	r     resource.Resource
	ie    framesystem.InputEnabled
	steps [][]referenceframe.Input
}

// executeTeleop executes a trajectory by sending joint targets to all components.
// Arms go to their streams when the streamed executor is enabled and available for them;
// everything else goes through the unary path, in parallel across components. Unlike
// ms.execute, it skips the step-0 position check (which blocks on CurrentInputs gRPC calls).
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

	// Route each component. Streamed pushes happen inline: they are cheap and mostly
	// non-blocking, and their bookkeeping is executor-goroutine state.
	var unary []unaryJob
	var errs []error
	for name, steps := range perComponent {
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

		if armComp, ok := r.(arm.Arm); ok && tp.streamed {
			if !tp.hasArmState(name) && !stepsMove(traj[0][name], steps) {
				// The plan covers every arm in the frame system. Do not open a stream to one
				// that teleop is not moving.
				continue
			}
			sent, err := tp.executeStreamed(ctx, name, armComp, steps)
			switch {
			case errors.Is(err, errTeleopStreamFallback):
				// Streaming is not available for this arm; the unary path below takes it.
			case err != nil:
				errs = append(errs, fmt.Errorf("teleop stream for %q: %w", name, err))
				continue
			default:
				if sent {
					tp.commandedJoints[name] = steps[len(steps)-1]
				}
				continue
			}
		}
		unary = append(unary, unaryJob{name: name, r: r, ie: ie, steps: steps})
	}

	if len(unary) > 0 {
		errs = append(errs, tp.executeUnary(ctx, ms, unary)...)
	}
	return errors.Join(errs...)
}

// executeUnary sends each job's steps to its component with one blocking call per component,
// all components in parallel, after EMA-smoothing the steps. It returns one error per failed
// component.
func (tp *teleopPipeline) executeUnary(ctx context.Context, ms *builtIn, jobs []unaryJob) []error {
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

	var wg sync.WaitGroup
	errs := make([]error, len(jobs))
	for i, job := range jobs {
		// Apply EMA smoothing to joint positions before sending (executor goroutine only).
		smoothed := make([][]referenceframe.Input, len(job.steps))
		prev := tp.commandedJoints[job.name]
		for k, step := range job.steps {
			smoothed[k] = emaSmooth(step, prev, smoothAlpha)
			prev = smoothed[k]
		}
		tp.commandedJoints[job.name] = prev

		wg.Add(1)
		go func(i int, job unaryJob, smoothed [][]referenceframe.Input) {
			defer wg.Done()
			var err error
			if armComp, ok := job.r.(arm.Arm); ok {
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
				err = job.ie.GoToInputs(ctx, smoothed...)
			}
			if err != nil {
				if actuator, ok := job.r.(inputEnabledActuator); ok {
					//nolint:errcheck
					_ = actuator.Stop(context.WithoutCancel(ctx), nil)
				}
				errs[i] = err
			}
		}(i, job, smoothed)
	}
	wg.Wait()

	var out []error
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	return out
}

// executeStreamed pushes one arm's planned steps into its stream as joint targets, opening or
// reopening the stream first if needed. It returns sent=false with a nil error when the
// trajectory was deliberately not sent: the stream is already too far ahead of the arm, or the
// pipeline is stopping. It returns errTeleopStreamFallback once streaming has been
// given up for this arm, so the caller can use the unary path for it instead.
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
	if comp.streamFallbackUnary {
		return false, errTeleopStreamFallback
	}

	s := comp.stream
	if s != nil && s.isDone() {
		// The stream died since the last cycle (arm fault, Stop from elsewhere, transport error).
		// Run has already stopped the arm; decide whether to reopen or give up.
		comp.stream = nil
		tp.noteStreamFailure(ctx, comp, s)
		if comp.streamFallbackUnary {
			return false, errTeleopStreamFallback
		}
		s = nil
	}
	if s == nil {
		var err error
		s, err = tp.openArmStream(ctx, a, comp)
		if err != nil {
			return false, fmt.Errorf("opening arm stream: %w", err)
		}
	}

	// Latest-wins: while the stream holds more unsent trajectory than maxLead, pushing more
	// would only make the arm lag the operator further. Skip this trajectory, leave the planning
	// head where it is so the next plan still starts from the last committed target, and arm a
	// replan for when the lead should have shrunk enough (in case no new pose arrives to do it).
	if lead := s.trajexRunway(); lead > tp.maxLead {
		comp.streamSkipped++
		tp.scheduleRetry(lead - tp.maxLead)
		tp.logger.CDebugf(ctx, "teleop executor: %q stream is %s ahead of the arm (max %s); skipping trajectory",
			name, lead, tp.maxLead)
		return false, nil
	}

	for _, step := range steps {
		if err := s.push(ctx, step); err != nil {
			if errors.Is(err, errTeleopStreamClosed) && s.isDone() {
				// Run died while we were pushing. Record it now so the next cycle reopens (or
				// falls back) immediately rather than discovering the dead stream a cycle late.
				comp.stream = nil
				tp.noteStreamFailure(ctx, comp, s)
				if comp.streamFallbackUnary {
					return false, errTeleopStreamFallback
				}
			}
			return false, err
		}
	}
	return true, nil
}

// openArmStream reads the arm's current joint positions, starts a stream seeded from them, and
// moves the arm's planning head onto that seed so the next plan starts from where the arm
// actually is. Caller holds comp.mu and runs on the executor goroutine.
func (tp *teleopPipeline) openArmStream(ctx context.Context, a arm.Arm, comp *teleopArmState) (*teleopArmStream, error) {
	seed, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reading seed joint positions: %w", err)
	}

	streamCtx, cancel := context.WithCancel(tp.streamCtx)
	s := &teleopArmStream{
		targets: make(chan []referenceframe.Input, teleopStreamTargetBuffer),
		done:    make(chan struct{}),
		diag:    diagnostics.New(teleopStreamDiagWindow),
		cancel:  cancel,
		started: time.Now(),
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
		// Run blocks on every acknowledgment, so they are drained even though nothing reads them.
		acks := make(chan motion.TempStreamResponse)
		acksDrained := make(chan struct{})
		go func() {
			defer close(acksDrained)
			for range acks {
			}
		}()
		err := tp.runArmStream(streamCtx, tp.logger, a, tp.streamOpts, s.targets, seed, s.diag, acks)
		close(acks)
		<-acksDrained
		s.err = err
		close(s.done)
		if err != nil && !errors.Is(err, context.Canceled) {
			tp.logger.CWarnf(streamCtx, "teleop arm stream for %q ended: %v", name, err)
		}
		tp.logger.Infow("teleop arm stream stats", "component", name, "options", tp.streamOpts, "stats", s.diag.Stats())
	}()
	tp.logger.CInfof(ctx, "teleop executor: opened arm stream for %q", name)
	return s, nil
}

// noteStreamFailure records that comp's stream s has died and decides whether streaming should
// be retried for this arm or abandoned for the rest of the session. Caller holds comp.mu.
func (tp *teleopPipeline) noteStreamFailure(ctx context.Context, comp *teleopArmState, s *teleopArmStream) {
	err := s.err
	comp.streamRestarts++
	if time.Since(s.started) >= teleopStreamHealthyAfter {
		comp.streamFails = 0
	}
	comp.streamFails++

	var reason string
	switch {
	case status.Code(err) == codes.Unimplemented:
		reason = "arm does not implement MoveThroughJointPositionsStreamed"
	case s.pushed.Load() == 0:
		// Dying before accepting a single target means streaming cannot start at all here
		// (e.g. a build without trajex), not that the arm hit a transient fault.
		reason = fmt.Sprintf("stream ended before accepting any target: %v", err)
	case comp.streamFails >= teleopStreamMaxConsecutiveFails:
		reason = fmt.Sprintf("%d consecutive stream failures, last: %v", comp.streamFails, err)
	}
	if reason != "" {
		comp.streamFallbackUnary = true
		comp.streamFallbackReason = reason
		tp.logger.CWarnf(ctx, "teleop executor: giving up on streaming to %q, using unary moves: %s", comp.name, reason)
		return
	}
	tp.logger.CWarnf(ctx, "teleop executor: arm stream for %q died (%d consecutive), will reopen: %v",
		comp.name, comp.streamFails, err)
}

// armState returns the streamed-executor state for the named arm, creating it on first use. It
// returns nil once the pipeline is stopping.
func (tp *teleopPipeline) armState(name string) *teleopArmState {
	tp.armsMu.Lock()
	defer tp.armsMu.Unlock()
	if tp.streamsStopped {
		return nil
	}
	st, ok := tp.arms[name]
	if !ok {
		st = &teleopArmState{name: name}
		tp.arms[name] = st
	}
	return st
}

func (tp *teleopPipeline) hasArmState(name string) bool {
	tp.armsMu.Lock()
	defer tp.armsMu.Unlock()
	_, ok := tp.arms[name]
	return ok
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

// stepsMove reports whether any step differs from start.
func stepsMove(start []referenceframe.Input, steps [][]referenceframe.Input) bool {
	for _, step := range steps {
		if len(step) != len(start) || referenceframe.InputsL2Distance(step, start) > 0 {
			return true
		}
	}
	return false
}

// scheduleRetry arms a one-shot replan after `after`, if none is already pending.
func (tp *teleopPipeline) scheduleRetry(after time.Duration) {
	if !tp.retryPending.CompareAndSwap(false, true) {
		return
	}
	if after < teleopStreamMinRetry {
		after = teleopStreamMinRetry
	}
	time.AfterFunc(after, func() {
		tp.retryPending.Store(false)
		trySendNotify(tp.notify)
	})
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
					// The pipeline is stopping; the error is just our own cancellation.
					return
				}
				tp.lastErr.Store(&err)
				tp.logger.CWarnf(ctx, "teleop executor error: %v", err)
				tp.resetPlanningHead(ctx, ms)
			} else {
				tp.lastErr.Store(nil)
				// Update planning head to what was actually commanded (EMA output or last
				// pushed target) so the next plan starts from reality.
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
}

// stop shuts down the pipeline: the planner and executor first, so nothing pushes to a stream
// while it is closing, then every arm stream, gracefully where possible.
func (tp *teleopPipeline) stop(ctx context.Context, ms *builtIn) {
	tp.workers.Stop()
	tp.closeStreams(ctx)
}

// closeStreams ends every open arm stream. Each is first half-closed so Run sends trajex's
// remaining trajectory to the arm and waits for it to finish; streams still running after
// teleopStreamCloseTimeout are cancelled, which makes Run stop their arms instead.
func (tp *teleopPipeline) closeStreams(ctx context.Context) {
	tp.armsMu.Lock()
	tp.streamsStopped = true
	tp.armsMu.Unlock()
	tp.closeArmStreams()

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
	// Cancel regardless: a stream that already returned ignores it, and one still draining is
	// cut off here, with Run stopping its arm.
	tp.streamCancel()
	select {
	case <-streamsDone:
	case <-time.After(teleopStreamCloseTimeout):
		// Run is stuck waiting for an arm to stop. Leave its goroutine to finish on its own
		// rather than hold up a reconfigure or close.
		tp.logger.CErrorf(ctx, "teleop arm streams still running %s after cancel; abandoning them", teleopStreamCloseTimeout)
	}
}

// closeArmStreams half-closes every open arm stream, so each arm finishes what it was sent. The
// executor opens a new stream for an arm if a later trajectory still moves it.
func (tp *teleopPipeline) closeArmStreams() {
	for _, st := range tp.armStates() {
		st.mu.Lock()
		s := st.stream
		st.stream = nil
		st.mu.Unlock()
		if s != nil {
			s.close()
		}
	}
}

// streamStatus reports the streamed executor's per-arm state for teleop_status.
func (tp *teleopPipeline) streamStatus() map[string]any {
	comps := tp.armStates()
	streams := make(map[string]any, len(comps))
	for _, comp := range comps {
		comp.mu.Lock()
		info := map[string]any{
			"open":           comp.stream != nil && !comp.stream.isDone(),
			"restarts":       comp.streamRestarts,
			"skipped":        comp.streamSkipped,
			"fallback_unary": comp.streamFallbackUnary,
		}
		if comp.streamFallbackReason != "" {
			info["fallback_reason"] = comp.streamFallbackReason
		}
		if s := comp.stream; s != nil {
			info["trajex_runway_ms"] = float64(s.trajexRunway().Microseconds()) / 1e3
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

		streamed := true
		var streamConf TeleopStreamConfig
		if ms.conf != nil {
			streamed = ms.conf.TeleopExecutor != TeleopExecutorUnary
			streamConf = ms.conf.TeleopStream
		}
		runArmStream := ms.teleopRunArmStream
		if runArmStream == nil {
			runArmStream = streaming.Run
		}
		ms.mu.RUnlock()

		streamCtx, streamCancel := context.WithCancel(context.Background())
		tp := &teleopPipeline{
			logger:           ms.logger.Sublogger("teleop"),
			cachedFrameSys:   frameSys,
			cachedBaseInputs: fsInputs,
			streamed:         streamed,
			streamOpts:       streamConf.streamOptions(),
			maxLead:          streamConf.maxLead(),
			runArmStream:     runArmStream,
			streamCtx:        streamCtx,
			streamCancel:     streamCancel,
			components:       make(map[string]*teleopComponent),
			notify:           make(chan struct{}, 1),
			trajCh:           make(chan motionplan.Trajectory, 1),
			planningHead:     fsInputs,
			commandedJoints:  make(map[string][]referenceframe.Input),
			arms:             make(map[string]*teleopArmState),
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
		return
	}
	// The removed component's arm is not known here (the component may be a frame the arm
	// carries), so every stream is drained. The executor reopens one for each arm still moving.
	tp.closeArmStreams()
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

		executor := TeleopExecutorUnary
		if tp.streamed {
			executor = TeleopExecutorStreamed
		}
		status := map[string]any{
			"running":           true,
			"executor":          executor,
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
		if tp.streamed {
			status["streams"] = tp.streamStatus()
		}

		resp[DoTeleopStatus] = status
		return resp, true, nil
	}

	return resp, false, nil
}
