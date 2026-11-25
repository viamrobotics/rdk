package servo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/spatialmath"
)

// Streamer is the loop's output; *streaming.ArmStreamer satisfies it.
type Streamer interface {
	Send(ctx context.Context, pts []streaming.Point) error
	Runway() time.Duration
	Done() <-chan struct{}
	Close() error
}

// ReadJoints returns the arm's measured joints (rad); the glue passes arm.JointPositions, tests a fake.
type ReadJoints func(ctx context.Context) ([]float64, error)

// State values mirror the proto's ServoState.
type State string

// State values.
const (
	StateRunning        State = "running"
	StateConverged      State = "converged"
	StateStalled        State = "stalled"
	StateStopped        State = "stopped"
	StateStoppedTimeout State = "stopped_timeout"
	StateFaulted        State = "faulted"
)

const (
	defaultLead   = 5 * time.Millisecond // diff-ik DefaultLead
	minHold       = 100 * time.Millisecond
	maxTwistHold  = 200 * time.Millisecond
	leashS        = 0.1 // the target may lead FK(q_cmd) by this many seconds at the speed limits
	maxSolverFail = 3   // consecutive solver failures that fault the loop
	tolMM, tolDeg = 2.0, 2.0
	tickWindow    = 100
)

// errDone is returned by Tick once the loop is in a terminal state.
var errDone = errors.New("servo loop done")

// LoopConfig holds the loop's knobs. Zero values take the defaults noted.
type LoopConfig struct {
	RateHz    float64       // default 50, lowered to the arm's operating_freq_hz by the glue
	Timeout   time.Duration // dead-man; required
	Ramp      time.Duration // stop ramp, 0 to 1 s
	Gain      float64       // 1/s; default 2
	Task      Task
	Lead      time.Duration // how far ahead each point is stamped; default 5 ms
	Grace     time.Duration // stream-end grace before FAULTED; default 100 ms
	ResyncDeg float64       // 0: 3 x the largest joint velocity cap (deg/s) / RateHz
	// T0 is the stream time origin: the moment the glue sent the t=0 hold point. Every point is stamped
	// now-T0+Lead, so times stay strictly increasing. Zero: the clock at NewLoop.
	T0 time.Time
	// TickTimeout bounds one Run tick's read and send; a hang past it faults the loop. Default max(5 periods, 500 ms).
	TickTimeout time.Duration
	// CloseTimeout bounds Run's wait for the streamer's Close (which waits on the arm's RPC). Default 2 s.
	CloseTimeout time.Duration
}

// Limit names one reason the loop could not follow the command at full rate.
type Limit string

// The Limit values.
const (
	LimitJointPosition Limit = "joint_position"
	LimitJointSpeed    Limit = "joint_speed"
	LimitTCPSpeed      Limit = "tcp_speed"
	LimitSingularity   Limit = "singularity"
)

// Status is a snapshot for replies, servo_status and the benchmark.
type Status struct {
	State             State
	QCmd, QMeas       []float64 // rad; QMeas from the last ReadJoints
	ErrMM, ErrDeg     float64   // measured tool pose vs target (tool-axis angle for TaskPointing)
	Limited           bool      // the last tick's step was limited
	LimitedBy         []Limit   // what limited the last tick, in the order of the Limit constants; empty when nothing did
	Damping           float64
	Ticks, Overruns   int
	SolverFailures    int
	Resyncs           int
	Limits            Limits // effective limits, for servo_status
	LastTick, TickP99 time.Duration
}

// Loop is the 50 Hz servo loop: Tick reads the arm, advances the target, asks the Solver for a joint
// step, limits it and streams one point. SetTarget, SetVelocity, BeginStop and Status may be called
// from other goroutines; Tick and Run belong to one.
type Loop struct {
	model   referenceframe.Model
	solver  Solver
	limits  Limits
	out     Streamer
	read    ReadJoints
	clock   func() time.Time
	cfg     LoopConfig
	dt      float64
	q0      []float64        // the start joints, immutable
	target0 spatialmath.Pose // FK(q0), immutable

	mu         sync.Mutex
	target     spatialmath.Pose // mm, base frame
	ff         [6]float64
	holdAfter  time.Duration
	lastSet    time.Time // last SetTarget/SetVelocity: ff hold
	lastVel    time.Time // last SetVelocity
	lastUpdate time.Time // dead-man
	lastReach  float64   // rad; pointing leash
	stopping   bool
	rampStart  time.Time
	final      State
	doneAt     time.Time // first tick that saw the stream end
	state      State
	terminal   bool
	cause      error
	done       chan struct{}
	errRing    []float64
	tickDurs   []time.Duration
	status     Status

	// Tick-goroutine only.
	qCmd, lastStep []float64
	lastTime       time.Duration
	failStreak     int
}

// NewLoop builds a loop starting at q0, holding FK(q0) as its target until SetTarget.
func NewLoop(model referenceframe.Model, q0 []float64, read ReadJoints, s Solver, l Limits, out Streamer,
	clock func() time.Time, cfg LoopConfig,
) (*Loop, error) {
	n := len(model.DoF())
	if n == 0 {
		return nil, errors.New("loop: model has no joints")
	}
	if len(q0) != n || len(l.Lo) != n || len(l.Hi) != n || len(l.Vel) != n || len(l.Acc) != n {
		return nil, fmt.Errorf("loop: model has %d joints, got q0 %d, limits %d/%d/%d/%d",
			n, len(q0), len(l.Lo), len(l.Hi), len(l.Vel), len(l.Acc))
	}
	if cfg.RateHz == 0 {
		cfg.RateHz = 50
	}
	if cfg.Gain == 0 {
		cfg.Gain = 2
	}
	if cfg.Lead == 0 {
		cfg.Lead = defaultLead
	}
	if cfg.Grace == 0 {
		cfg.Grace = 100 * time.Millisecond
	}
	if cfg.ResyncDeg == 0 {
		cfg.ResyncDeg = 3 * slices.Max(l.Vel) * 180 / math.Pi / cfg.RateHz
	}
	if cfg.TickTimeout == 0 {
		cfg.TickTimeout = max(5*time.Duration(float64(time.Second)/cfg.RateHz), 500*time.Millisecond)
	}
	if cfg.CloseTimeout == 0 {
		cfg.CloseTimeout = 2 * time.Second
	}
	if cfg.T0.IsZero() {
		cfg.T0 = clock()
	}
	switch {
	case !(cfg.RateHz >= 1), cfg.TickTimeout < 0, cfg.CloseTimeout < 0, !(cfg.Gain > 0), !(cfg.ResyncDeg > 0), cfg.Lead < 0, cfg.Grace < 0:
		return nil, fmt.Errorf("loop: bad config %+v", cfg)
	case cfg.Timeout <= 0:
		return nil, errors.New("loop: timeout must be > 0")
	case cfg.Ramp < 0 || cfg.Ramp > time.Second:
		return nil, fmt.Errorf("loop: ramp %v outside [0, 1s]", cfg.Ramp)
	case read == nil || s == nil || out == nil || clock == nil:
		return nil, errors.New("loop: read, solver, streamer and clock are required")
	}
	p, q, err := FK(model, q0)
	if err != nil {
		return nil, err
	}
	qq := spatialmath.Quaternion(q)
	now := clock()
	lp := &Loop{
		model: model, solver: s, limits: l, out: out, read: read, clock: clock, cfg: cfg, dt: 1 / cfg.RateHz,
		target: spatialmath.NewPose(p.Mul(mmPerM), &qq), lastUpdate: now, state: StateRunning,
		done: make(chan struct{}), q0: slices.Clone(q0), qCmd: slices.Clone(q0), lastStep: make([]float64, n),
	}
	lp.target0 = lp.target
	lp.status.Limits = l
	return lp, nil
}

// SetTarget sets the target pose (mm, base frame) and feed-forward twist ff (m/s, rad/s); it refreshes
// the dead-man. The target advances at ff for max(2 x the interval since the last call, 100 ms), then holds.
func (l *Loop) SetTarget(p spatialmath.Pose, ff [6]float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopping || l.terminal {
		return false
	}
	now := l.clock()
	l.holdAfter = minHold
	if !l.lastSet.IsZero() {
		l.holdAfter = max(minHold, 2*now.Sub(l.lastSet))
	}
	l.lastSet, l.lastUpdate = now, now
	l.target, l.ff = p, ff
	l.errRing = l.errRing[:0]
	return true
}

// SetVelocity advances the current target at v (m/s, rad/s) until min(2 x the interval since the previous
// SetVelocity, 200 ms) has passed (200 ms for the first), then holds it: a twist expires unless resent.
// It refreshes the dead-man. Both setters return false, changing nothing, once the loop is stopping or terminal.
func (l *Loop) SetVelocity(v [6]float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopping || l.terminal {
		return false
	}
	now := l.clock()
	l.holdAfter = maxTwistHold
	if !l.lastVel.IsZero() {
		l.holdAfter = min(maxTwistHold, 2*now.Sub(l.lastVel))
	}
	l.lastVel, l.lastSet, l.lastUpdate = now, now, now
	l.ff = v
	return true
}

// BeginStop starts a ramp to zero ending in final (StateStopped or StateStoppedTimeout; anything else
// becomes StateStopped). A second call keeps the first deadline and only
// upgrades final to StateStopped. It never blocks.
func (l *Loop) BeginStop(final State) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.beginStopLocked(final)
}

func (l *Loop) beginStopLocked(final State) {
	if final != StateStopped && final != StateStoppedTimeout {
		final = StateStopped
	}
	if !l.stopping {
		l.stopping, l.rampStart, l.final = true, l.clock(), final
		return
	}
	if final == StateStopped {
		l.final = final
	}
}

// initialTarget is the pose the loop was built to hold (FK(q0), mm, base frame).
func (l *Loop) initialTarget() spatialmath.Pose { return l.target0 }

func (l *Loop) isStopping() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stopping
}

// Err is the cause of a FAULTED end, or nil.
func (l *Loop) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != StateFaulted {
		return nil
	}
	return l.cause
}

// Done is closed when the loop reaches a terminal state.
func (l *Loop) Done() <-chan struct{} { return l.done }

// finish enters a terminal state. Caller holds l.mu.
func (l *Loop) finish(s State, cause error) error {
	l.terminate(s, cause)
	return errDone
}

func (l *Loop) terminate(s State, cause error) {
	if !l.terminal {
		l.terminal, l.state, l.cause = true, s, cause
		l.status.State = s
		close(l.done)
	}
}

// Status returns a snapshot.
func (l *Loop) Status() Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := l.status
	s.State = l.state
	s.QCmd, s.QMeas = slices.Clone(s.QCmd), slices.Clone(s.QMeas)
	s.LimitedBy = slices.Clone(s.LimitedBy)
	s.Limits.Lo, s.Limits.Hi = slices.Clone(s.Limits.Lo), slices.Clone(s.Limits.Hi)
	s.Limits.Vel, s.Limits.Acc = slices.Clone(s.Limits.Vel), slices.Clone(s.Limits.Acc)
	if n := len(l.tickDurs); n > 0 {
		d := slices.Clone(l.tickDurs)
		slices.Sort(d)
		s.TickP99 = d[n*99/100]
	}
	return s
}

// rampScale reports whether a stop ramp is active and the velocity scale in [0, 1]. Caller holds l.mu.
func (l *Loop) rampScale() (bool, float64) {
	if !l.stopping {
		return false, 1
	}
	if l.cfg.Ramp <= 0 {
		return true, 0
	}
	return true, math.Max(0, 1-float64(l.clock().Sub(l.rampStart))/float64(l.cfg.Ramp))
}

// canStop reports whether the last step is small enough to stop within the acceleration limit.
func (l *Loop) canStop() bool {
	for i, d := range l.lastStep {
		if math.Abs(d) > l.limits.Acc[i]*l.dt*l.dt {
			return false
		}
	}
	return true
}

// Tick runs one control step. It returns errDone once the loop is terminal (the cause, if a fault, is
// what Run returns) and nil otherwise.
func (l *Loop) Tick(ctx context.Context) error {
	start := l.clock()

	// 0. terminal, stream end, dead-man, ramp complete
	l.mu.Lock()
	if l.terminal {
		l.mu.Unlock()
		return errDone
	}
	select {
	case <-l.out.Done():
		defer l.mu.Unlock()
		if l.stopping { // the arm ended the stream under a stop: nothing left to ramp
			return l.finish(l.final, nil)
		}
		if l.doneAt.IsZero() {
			l.doneAt = start
		}
		if start.Sub(l.doneAt) >= l.cfg.Grace {
			return l.finish(StateFaulted, errors.New("arm ended the stream"))
		}
		return nil // wait out the grace period for a BeginStop
	default:
	}
	if !l.stopping && start.Sub(l.lastUpdate) > l.cfg.Timeout {
		l.beginStopLocked(StateStoppedTimeout)
	}
	if ramping, scale := l.rampScale(); ramping && scale == 0 && l.canStop() {
		defer l.mu.Unlock()
		return l.finish(l.final, nil)
	}
	l.mu.Unlock()

	// 1. measured joints, then a plain resync
	qm, err := l.read(ctx)
	if err == nil && len(qm) != len(l.qCmd) {
		err = fmt.Errorf("arm reported %d joints, model has %d", len(qm), len(l.qCmd))
	}
	if err == nil && slices.ContainsFunc(qm, func(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) }) {
		err = errors.New("arm reported a non-finite joint position")
	}
	if err != nil {
		return l.fault(ctx, fmt.Errorf("reading joints: %w", err))
	}
	resynced := false
	for i := range qm {
		if math.Abs(qm[i]-l.qCmd[i]) > l.cfg.ResyncDeg*math.Pi/180 {
			copy(l.qCmd, qm)
			resynced = true
			break
		}
	}

	// 2. FK(q_cmd), then advance the target under mu
	cp, cq, err := FK(l.model, l.qCmd)
	if err != nil {
		return l.fault(ctx, err)
	}
	l.mu.Lock()
	ramping, scale := l.rampScale()
	target, ff := l.advance(cp, cq, ramping)
	l.mu.Unlock()

	// 3. solve, scale by the ramp, limit. A failed tick runs zero dq through Limits.Apply: from motion this
	// brakes within the acceleration limit rather than holding q_cmd literally (a literal hold would break it).
	dq, info, serr := l.solver.Step(StepInput{
		Model: l.model, Q: l.qCmd, Target: target, FF: ff, Gain: l.cfg.Gain,
		Dt: l.dt, Task: l.cfg.Task, Limits: l.limits,
	})
	if serr == nil && (len(dq) != len(l.qCmd) || slices.ContainsFunc(dq, func(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) })) {
		serr = fmt.Errorf("%s returned a non-finite or mis-sized step", l.solver.Name())
	}
	failed := serr != nil
	if failed {
		l.failStreak++
		dq, info = make([]float64, len(l.qCmd)), StepInfo{}
	} else {
		l.failStreak = 0
		dq = slices.Clone(dq)
		for i := range dq {
			dq[i] *= scale
		}
	}
	step, by := l.limits.Apply(l.qCmd, dq, l.lastStep, l.dt)
	limited := by != 0
	// 4. send one point
	pt := streaming.Point{
		Positions: make([]float64, len(step)), Velocities: make([]float64, len(step)), Accelerations: make([]float64, len(step)),
	}
	for i, s := range step {
		pt.Positions[i], pt.Velocities[i], pt.Accelerations[i] = l.qCmd[i]+s, s/l.dt, (s-l.lastStep[i])/(l.dt*l.dt)
	}
	pt.Time = max(start.Sub(l.cfg.T0)+l.cfg.Lead, l.lastTime+time.Microsecond)
	if err := l.out.Send(ctx, []streaming.Point{pt}); err != nil {
		select {
		case <-l.out.Done(): // the arm ended the stream: the next tick runs the grace logic
			return nil
		default:
		}
		return l.fault(ctx, fmt.Errorf("sending joints: %w", err))
	}
	l.lastTime = pt.Time
	copy(l.qCmd, pt.Positions)
	copy(l.lastStep, step)
	if failed && l.failStreak >= maxSolverFail { // the braking point is sent first
		l.mu.Lock()
		defer l.mu.Unlock()
		l.status.SolverFailures++
		return l.finish(StateFaulted, fmt.Errorf("solver failed %d ticks in a row: %w", l.failStreak, serr))
	}

	// 5. measured error and state
	ms, err := l.measure(target, qm)
	if err != nil {
		return l.fault(ctx, err)
	}
	d := l.clock().Sub(start)
	l.mu.Lock()
	defer l.mu.Unlock()
	st := &l.status
	if failed {
		st.SolverFailures++
	}
	if resynced {
		st.Resyncs++
	}
	st.Ticks++
	st.LastTick = d
	if d > time.Duration(l.dt*float64(time.Second)) {
		st.Overruns++
	}
	if len(l.tickDurs) == tickWindow {
		l.tickDurs = l.tickDurs[1:]
	}
	l.tickDurs = append(l.tickDurs, d)
	st.ErrMM, st.ErrDeg = ms.posMM, ms.angDeg
	st.Damping, st.Limited = info.Damping, limited
	st.LimitedBy = nil
	for _, c := range []struct {
		on bool
		by Limit
	}{
		{by&LimitedJointPosition != 0, LimitJointPosition},
		{by&LimitedJointSpeed != 0, LimitJointSpeed},
		{info.TCPCapped, LimitTCPSpeed},
		{info.Damping > 0, LimitSingularity},
	} {
		if c.on {
			st.LimitedBy = append(st.LimitedBy, c.by)
		}
	}
	st.QCmd, st.QMeas = slices.Clone(l.qCmd), slices.Clone(qm)
	l.lastReach = ms.convDeg * math.Pi / 180
	l.state = l.nextState(ms, limited || info.Damping > 0, ff != [6]float64{})
	return nil
}

// fault ends the loop in StateFaulted, unless ctx was cancelled (the caller's stop, not a fault). A
// deadline (Run's per-tick timeout on a hung read or send) is a fault.
func (l *Loop) fault(ctx context.Context, cause error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.finish(StateFaulted, cause)
}

// advance moves the stored target one tick along its feed-forward twist and returns it and the twist in
// force. cp, cq are FK(q_cmd). Caller holds l.mu.
//   - After holdAfter the target stays put and the twist is zero.
//   - Ramping: the target stays put; the solver's step is scaled toward zero instead.
//   - Leash, per axis group: an advance is skipped only when the error already exceeds leashS x the speed
//     limit and the advance would grow it (vel . e > 0).
func (l *Loop) advance(cp r3.Vector, cq quat.Number, ramping bool) (spatialmath.Pose, [6]float64) {
	var zero [6]float64
	vel := l.ff
	if l.clock().Sub(l.lastSet) > l.holdAfter { // ff holds until lastSet + holdAfter
		return l.target, zero
	}
	if ramping || vel == zero {
		return l.target, vel
	}
	tp, tq := l.target.Point().Mul(1/mmPerM), l.target.Orientation().Quaternion()
	e := poseError(tp, tq, cp, cq)
	vl, el := r3.Vector{X: vel[0], Y: vel[1], Z: vel[2]}, r3.Vector{X: e[0], Y: e[1], Z: e[2]}
	if !(el.Norm() > leashS*l.limits.MaxLinear && vl.Dot(el) > 0) {
		tp = tp.Add(vl.Mul(l.dt))
	}
	va, ea := r3.Vector{X: vel[3], Y: vel[4], Z: vel[5]}, r3.Vector{X: e[3], Y: e[4], Z: e[5]}
	angErr := ea.Norm()
	if l.cfg.Task == TaskPointing {
		// leash on the reachable part only, along the pointing rotation; roll is discarded
		ea, _, _ = pointingError(toolAxis(cq), toolAxis(tq))
		angErr = l.lastReach
		at := toolAxis(tq)
		va = va.Sub(at.Mul(va.Dot(at)))
	}
	if !(angErr > leashS*l.limits.MaxAngular && va.Dot(ea) > 0) {
		tq = quat.Mul(expRot(va.Mul(l.dt)), tq)
	}
	q := spatialmath.Quaternion(tq)
	l.target = spatialmath.NewPose(tp.Mul(mmPerM), &q)
	return l.target, vel
}

type measured struct {
	posMM, angDeg float64 // angDeg: pointing angle (pointing) or orientation error norm (pose)
	convDeg       float64 // angular error for convergence: reach (pointing) or orientation error (pose)
	stallNorm     float64
}

var unitWeights = [6]float64{1, 1, 1, 1, 1, 1}

// measure is the tool pose error against target from the measured joints qm.
func (l *Loop) measure(target spatialmath.Pose, qm []float64) (measured, error) {
	tp, tq := target.Point().Mul(1/mmPerM), target.Orientation().Quaternion()
	mp, mq, err := FK(l.model, qm)
	if err != nil {
		return measured{}, err
	}
	if l.cfg.Task != TaskPointing {
		em := poseError(tp, tq, mp, mq)
		norm := 0.0
		for _, v := range em {
			norm += v * v
		}
		ang := activeAngDeg(em, unitWeights)
		return measured{posMM: activePosMM(em, unitWeights), angDeg: ang, convDeg: ang, stallNorm: math.Sqrt(norm)}, nil
	}
	Jm, err := Jacobian(l.model, qm)
	if err != nil {
		return measured{}, err
	}
	r, u, v := pointingError(toolAxis(mq), toolAxis(tq))
	_, n := Jm.Dims()
	// Servos sag inside a limit the command sits at: lock on whichever of qm/qCmd is at the limit.
	qLock := slices.Clone(qm)
	for i, c := range l.qCmd {
		if c >= l.limits.Hi[i]-limitEps || c <= l.limits.Lo[i]+limitEps {
			qLock[i] = c
		}
	}
	reach, _, err := pointingReach(mat.DenseCopyOf(Jm.Slice(0, 3, 0, n)), pointingRows(Jm, u, v),
		[2]float64{u.Dot(r), v.Dot(r)}, qLock, l.limits.Lo, l.limits.Hi)
	if err != nil {
		return measured{}, err
	}
	e1 := tp.Sub(mp).Norm()
	return measured{
		posMM: e1 * mmPerM, angDeg: r.Norm() * 180 / math.Pi, convDeg: reach * 180 / math.Pi, stallNorm: math.Hypot(e1, reach),
	}, nil
}

// nextState picks converged, stalled or running. moving means a feed-forward twist is active.
// Caller holds l.mu.
func (l *Loop) nextState(ms measured, limited, moving bool) State {
	if !moving && ms.posMM <= tolMM && ms.convDeg <= tolDeg {
		l.errRing = l.errRing[:0]
		return StateConverged
	}
	window := int(l.cfg.RateHz) // 1 s of ticks
	if len(l.errRing) == window {
		l.errRing = l.errRing[1:]
	}
	l.errRing = append(l.errRing, ms.stallNorm)
	if limited && len(l.errRing) == window && ms.stallNorm > 0.99*l.errRing[0] {
		return StateStalled
	}
	return StateRunning
}

// Run ticks at RateHz until the loop is terminal or ctx is done (state StateStopped, returning ctx's
// error), never queueing missed ticks. Cancelling ctx stops immediately, without a ramp: sessions should
// BeginStop and wait on Done, cancelling only as a last resort.
//
// Done closes when the loop is terminal, before the stream is closed. The glue owns arm.Stop: callers that
// must stop the arm should do so when Done closes, and wait for Run only before reusing the arm. On the
// way out Run closes the streamer (this is the sending goroutine, as ArmStreamer.Close requires), waiting
// at most CloseTimeout, and returns the fault cause, if any.
func (l *Loop) Run(ctx context.Context) error {
	period := time.Duration(l.dt * float64(time.Second))
	t := time.NewTicker(period)
	defer t.Stop()
	var cause error
loop:
	for {
		select {
		case <-ctx.Done():
			l.mu.Lock()
			l.terminate(StateStopped, ctx.Err())
			l.mu.Unlock()
			break loop
		case <-t.C:
		}
		tctx, cancel := context.WithTimeout(ctx, l.cfg.TickTimeout)
		err := l.Tick(tctx)
		cancel()
		if errors.Is(err, errDone) {
			break
		}
		if err != nil { // ctx cancelled mid-tick
			continue
		}
		if l.Status().LastTick > period { // an overrun: drop the tick the ticker buffered
			select {
			case <-t.C:
			default:
			}
		}
	}
	l.mu.Lock()
	cause = l.cause
	l.mu.Unlock()
	closed := make(chan error, 1)
	go func() { closed <- l.out.Close() }() // a hung driver can block Close; the goroutine is abandoned on timeout
	select {
	case cerr := <-closed:
		if cerr != nil {
			return errors.Join(cause, cerr)
		}
	case <-time.After(l.cfg.CloseTimeout):
		return errors.Join(cause, errors.New("servo: closing arm stream timed out"))
	}
	return cause
}
