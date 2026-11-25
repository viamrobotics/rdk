package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/servo"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Servo DoCommand verbs. Each is a key whose value is the request object, like stream_status.
const (
	DoServoStart    = "servo_start"
	DoServoTarget   = "servo_target"
	DoServoVelocity = "servo_velocity"
	DoServoStop     = "servo_stop"
	DoServoStatus   = "servo_status"
)

const (
	servoProbeWait   = 50 * time.Millisecond
	servoMaxRateHz   = 50
	servoArmStopWait = 2 * time.Second
	servoCloseWait   = 2 * time.Second // the loop's CloseTimeout
	servoProbeMax    = 2 * time.Second
	deg2rad          = math.Pi / 180
)

func (ms *builtIn) handleServoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, bool, error) {
	for _, v := range []struct {
		verb string
		fn   func(context.Context, map[string]interface{}) (map[string]interface{}, error)
	}{
		{DoServoStart, ms.servoStart},
		{DoServoTarget, ms.servoTarget},
		{DoServoVelocity, ms.servoVelocity},
		{DoServoStop, ms.servoStop},
		{DoServoStatus, ms.servoStatus},
	} {
		req, ok := cmd[v.verb]
		if !ok {
			continue
		}
		m, err := utils.AssertType[map[string]interface{}](req)
		if err != nil {
			return nil, true, fmt.Errorf("%s expects an object", v.verb)
		}
		resp, err := v.fn(ctx, m)
		return resp, true, err
	}
	return nil, false, nil
}

// waitEntry waits for a session's cleanup to finish, bounded by the entry's stop budget.
func waitEntry(ctx context.Context, e *servoEntry) error {
	t := time.NewTimer(e.stopBudget)
	defer t.Stop()
	select {
	case <-e.done:
		return nil
	case <-t.C:
		return fmt.Errorf("servo session on arm %q did not finish stopping in time", e.arm)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// waitServos waits for each stopping session's cleanup, each bounded by its own stop budget.
func (ms *builtIn) waitServos(ctx context.Context, es []*servoEntry) error {
	var errs []error
	for _, e := range es {
		if err := waitEntry(ctx, e); err != nil {
			ms.logger.Warnf("servo session %s: %v", e.id, err)
			errs = append(errs, err)
			if ctx.Err() != nil {
				break
			}
		}
	}
	return errors.Join(errs...)
}

// --- request parsing ---

func num(m map[string]interface{}, key string, def float64) (float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return def, nil
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	}
	return 0, fmt.Errorf("%s must be a number", key)
}

func nums(m map[string]interface{}, key string) ([]float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	if f, ok := v.([]float64); ok {
		return f, nil
	}
	s, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of numbers", key)
	}
	out := make([]float64, len(s))
	for i, e := range s {
		f, err := num(map[string]interface{}{key: e}, key, 0)
		if err != nil {
			return nil, fmt.Errorf("%s must be an array of numbers", key)
		}
		out[i] = f
	}
	return out, nil
}

func obj(m map[string]interface{}, key string) (map[string]interface{}, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	o, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return o, nil
}

// vec reads {x, y, z} (absent: zero), scaled by k.
func vec(m map[string]interface{}, key string, k float64) (r3.Vector, error) {
	o, err := obj(m, key)
	if err != nil {
		return r3.Vector{}, err
	}
	var v [3]float64
	for i, c := range []string{"x", "y", "z"} {
		if v[i], err = num(o, c, 0); err != nil {
			return r3.Vector{}, fmt.Errorf("%s.%w", key, err)
		}
	}
	return r3.Vector{X: v[0] * k, Y: v[1] * k, Z: v[2] * k}, nil
}

// seqOf reads seq: absent is nil; anything but a whole number >= 1 is an error.
func seqOf(m map[string]interface{}) (*int64, error) {
	v, ok := m["seq"]
	if !ok || v == nil {
		return nil, nil
	}
	f, err := num(m, "seq", 0)
	if err != nil || f != math.Trunc(f) || f < 1 || f > 1<<53 {
		return nil, errors.New("seq must be a whole number >= 1")
	}
	s := int64(f)
	return &s, nil
}

func frameOf(m map[string]interface{}) (string, error) {
	v, ok := m["reference_frame"]
	if !ok {
		return "world", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("reference_frame must be a string")
	}
	return s, nil
}

func toDeg(rad []float64) []float64 {
	out := make([]float64, len(rad))
	for i, r := range rad {
		out[i] = r / deg2rad
	}
	return out
}

func limitsReply(l servo.Limits) map[string]interface{} {
	out := map[string]interface{}{
		"min_deg": toDeg(l.Lo), "max_deg": toDeg(l.Hi),
		"max_vel_degs_per_sec_joints": toDeg(l.Vel), "max_acc_degs_per_sec2_joints": toDeg(l.Acc),
	}
	if !math.IsInf(l.MaxLinear, 0) {
		out["max_tcp_speed"] = l.MaxLinear
	}
	return out
}

// --- servo_start ---

// servoSetup is everything servo_start reads from the machine under ms.mu.
type servoSetup struct {
	a  arm.Arm
	fs *referenceframe.FrameSystem
}

func (ms *builtIn) servoSetup(ctx context.Context, name string) (servoSetup, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	r, ok := ms.components[name]
	if !ok {
		return servoSetup{}, fmt.Errorf("servo_start: no component named %q is known to the motion service", name)
	}
	a, err := utils.AssertType[arm.Arm](r)
	if err != nil {
		return servoSetup{}, fmt.Errorf("servo_start: component %q is not an arm: %w", name, err)
	}
	fs, err := ms.getFrameSystem(ctx, nil)
	if err != nil {
		return servoSetup{}, fmt.Errorf("servo_start: %w", err)
	}
	return servoSetup{a: a, fs: fs}, nil
}

// servoParams is everything a servo start needs, after parsing and defaults.
type servoParams struct {
	arm                   string
	mode                  servo.Mode
	opts                  arm.MoveOptions
	maxAngDegs, timeoutMS float64
	rampMS                float64
	stepMM, stepDeg       float64
	solver                string // "dls" or "solveonce"
	resyncDeg             float64
	streamed              bool // owned by a StreamServo call: the DoCommand target/velocity verbs refuse it
}

// servoStarted is what a start reports back.
type servoStarted struct {
	entry  *servoEntry
	limits servo.Limits
	rateHz float64
	solver string
}

// servoRanges are the numeric start parameters with their defaults and bounds, in validation order.
var servoRanges = []struct {
	key         string
	def, lo, hi float64
	loOK        bool // lo is allowed
}{
	{"max_angular_vel_degs_per_sec", 45, 0, math.Inf(1), false},
	{"timeout_ms", 500, 0, 10000, false},
	{"ramp_ms", 200, 0, 1000, true},
	{"max_target_step_mm", 100, 0, math.Inf(1), false},
	{"max_target_step_deg", 30, 0, math.Inf(1), false},
}

// parseServoParams reads the servo_start request map into servoParams, applying defaults.
func parseServoParams(req map[string]interface{}) (servoParams, error) {
	p := servoParams{}
	name, isStr := req["component_name"].(string)
	if v, present := req["component_name"]; present && v != nil && !isStr {
		return p, errors.New("servo_start: component_name must be a string")
	}
	p.arm = name
	p.mode = servo.Mode(fmt.Sprint(req["mode"]))
	var f [5]float64
	var err error
	for i, r := range servoRanges {
		if f[i], err = num(req, r.key, r.def); err != nil {
			return p, fmt.Errorf("servo_start: %w", err)
		}
	}
	p.maxAngDegs, p.timeoutMS, p.rampMS, p.stepMM, p.stepDeg = f[0], f[1], f[2], f[3], f[4]
	if p.opts, err = moveOptions(req); err != nil {
		return p, fmt.Errorf("servo_start: %w", err)
	}
	extra, err := obj(req, "extra")
	if err != nil {
		return p, fmt.Errorf("servo_start: %w", err)
	}
	return p, p.parseExtra(extra)
}

// parseExtra reads the solver and resync_deg options from an extra map.
func (p *servoParams) parseExtra(extra map[string]interface{}) error {
	var err error
	p.solver = "dls"
	if v, ok := extra["solver"]; ok && v != nil {
		if p.solver, ok = v.(string); !ok {
			return errors.New("servo_start: extra.solver must be a string")
		}
	}
	// 0 selects the loop's default.
	if p.resyncDeg, err = num(extra, "resync_deg", 0); err != nil {
		return fmt.Errorf("servo_start: extra.%w", err)
	}
	if v := extra["resync_deg"]; v != nil && p.resyncDeg == 0 {
		return fmt.Errorf("servo_start: extra.resync_deg must be > 0, got %v", p.resyncDeg)
	}
	return nil
}

// validate holds the range and enum checks shared by every way of starting a servo.
func (p servoParams) validate() error {
	if p.arm == "" {
		return errors.New("servo_start: component_name is required")
	}
	if p.mode != servo.ModePose && p.mode != servo.ModeVelocity {
		return errors.New(`servo_start: mode must be "pose" or "velocity"`)
	}
	for i, v := range []float64{p.maxAngDegs, p.timeoutMS, p.rampMS, p.stepMM, p.stepDeg} {
		r := servoRanges[i]
		if ok := v > r.lo || (r.loOK && v == r.lo); !ok || v > r.hi { // also rejects NaN
			cmp := ">"
			if r.loOK {
				cmp = ">="
			}
			return fmt.Errorf("servo_start: %s must be %s %v and <= %v, got %v", r.key, cmp, r.lo, r.hi, v)
		}
	}
	if err := validLimits(p.opts); err != nil {
		return err
	}
	if p.resyncDeg != 0 && !(p.resyncDeg > 0) { // also rejects NaN
		return fmt.Errorf("servo_start: extra.resync_deg must be > 0, got %v", p.resyncDeg)
	}
	if p.solver != "dls" && p.solver != "solveonce" {
		return fmt.Errorf(`servo_start: solver must be "dls" or "solveonce", got %q`, p.solver)
	}
	return nil
}

// validLimits checks that every limit in o (rad) is finite and >= 0, reporting degrees as the caller wrote them.
func validLimits(o arm.MoveOptions) error {
	ok := func(v float64) bool { return v >= 0 && !math.IsInf(v, 0) } // also rejects NaN
	for _, s := range []struct {
		key string
		v   float64
	}{{"max_vel_degs_per_sec", o.MaxVelRads}, {"max_acc_degs_per_sec2", o.MaxAccRads}} {
		if !ok(s.v) {
			return fmt.Errorf("servo_start: limits.%s must be finite and >= 0, got %v", s.key, s.v/deg2rad)
		}
	}
	for _, l := range []struct {
		key string
		vs  []float64
	}{{"max_vel_degs_per_sec_joints", o.MaxVelRadsJoints}, {"max_acc_degs_per_sec2_joints", o.MaxAccRadsJoints}} {
		for _, v := range l.vs {
			if !ok(v) {
				return fmt.Errorf("servo_start: limits.%s entries must be finite and >= 0, got %v", l.key, v/deg2rad)
			}
		}
	}
	if t := o.MaxTCPSpeedMPerSec; t != nil && !ok(*t) {
		return fmt.Errorf("servo_start: limits.max_tcp_speed must be finite and >= 0, got %v", *t)
	}
	return nil
}

func (ms *builtIn) servoStart(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	p, err := parseServoParams(req)
	if err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	st, err := ms.startServo(ctx, p)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session_id": st.entry.id, "limits": limitsReply(st.limits), "max_angular_vel_degs_per_sec": p.maxAngDegs,
		"timeout_ms": p.timeoutMS, "rate_hz": st.rateHz, "solver": st.solver,
	}, nil
}

// startServo claims the arm and starts a session from validated params.
func (ms *builtIn) startServo(ctx context.Context, p servoParams) (servoStarted, error) {
	name, mode, opts := p.arm, p.mode, p.opts
	maxAng, timeoutMS, rampMS, stepMM, stepDeg := p.maxAngDegs, p.timeoutMS, p.rampMS, p.stepMM, p.stepDeg
	resyncDeg := p.resyncDeg
	var solver servo.Solver
	var err error
	switch p.solver {
	case "dls":
		solver = servo.NewDLSSolver()
	case "solveonce":
		if solver, err = servo.NewSolveOnceSolver(ms.logger); err != nil {
			return servoStarted{}, fmt.Errorf("servo_start: solver solveonce is unavailable: %w", err)
		}
	default:
		return servoStarted{}, fmt.Errorf(`servo_start: solver must be "dls" or "solveonce", got %q`, p.solver)
	}

	// Claim the arm before touching it, so a concurrent start or TempStream never opens a second stream.
	// The entry is invisible to the verbs until published; every failure path below drops it.
	e := &servoEntry{done: make(chan struct{}), arm: name}
	if err := ms.reg.claim(name, holder{kind: holderServo, servo: e}); err != nil {
		return servoStarted{}, fmt.Errorf("servo_start: %w", err)
	}
	published := false
	defer func() {
		if !published {
			ms.reg.drop(name, e)
		}
	}()
	su, err := ms.servoSetup(ctx, name)
	if err != nil {
		return servoStarted{}, err
	}
	kin, err := su.a.Kinematics(ctx)
	if err != nil {
		return servoStarted{}, fmt.Errorf("servo_start: arm %q kinematics: %w", name, err)
	}
	kinConfig, err := json.Marshal(kin)
	if err != nil {
		return servoStarted{}, fmt.Errorf("servo_start: arm %q kinematics: %w", name, err)
	}
	model, ok := su.fs.Frame(name).(referenceframe.Model)
	if !ok {
		return servoStarted{}, fmt.Errorf("servo_start: frame %q is not an arm model", name)
	}
	dof := slices.Clone(model.DoF())
	own := kin.DoF()
	if len(dof) != len(own) {
		return servoStarted{}, fmt.Errorf("servo_start: frame system model has %d joints, arm reports %d", len(dof), len(own))
	}
	for i := range dof { // input_range_override may only narrow the arm's own range
		dof[i].Min, dof[i].Max = max(dof[i].Min, own[i].Min), min(dof[i].Max, own[i].Max)
		if dof[i].Min > dof[i].Max {
			return servoStarted{}, fmt.Errorf("servo_start: input_range_override for joint %d [%v, %v] lies outside the arm's range [%v, %v]",
				i, model.DoF()[i].Min, model.DoF()[i].Max, own[i].Min, own[i].Max)
		}
	}
	limits, err := servo.NewLimits(dof, opts, maxAng)
	if err != nil {
		return servoStarted{}, fmt.Errorf("servo_start: %w", err)
	}
	rate := float64(servoMaxRateHz)
	if sm, ok := kin.(*referenceframe.SimpleModel); ok {
		kp := sm.KinematicProperties()
		limits.TightenLinear(kp.MaxLinearTCPSpeedMMPerSec / 1000)
		if kp.OperatingFreqHz > 0 {
			rate = min(rate, kp.OperatingFreqHz)
		}
	}
	task := servo.TaskPose
	if len(dof) < 6 {
		task = servo.TaskPointing
	}

	q0, err := su.a.JointPositions(ctx, nil)
	if err != nil {
		return servoStarted{}, fmt.Errorf("servo_start: reading joints of %q: %w", name, err)
	}
	// The stream and loop run on a context detached from this request: only the session cancels it.
	sctx, cancel := context.WithCancel(context.Background())
	st := streaming.NewArmStreamer(sctx, su.a, diagnostics.New(0))
	t0 := time.Now()
	pctx, pcancel := context.WithTimeout(ctx, servoProbeMax)
	defer pcancel()
	probeErr := st.Send(pctx, []streaming.Point{probePoint(q0)})
	if probeErr == nil {
		select {
		case <-st.Done():
		case <-time.After(servoProbeWait):
		case <-pctx.Done():
			probeErr = pctx.Err()
		}
	}
	select {
	case <-st.Done():
		probeErr = errors.Join(probeErr, st.Close())
		cancel()
		return servoStarted{}, fmt.Errorf("servo_start: arm %q cannot stream (MoveThroughJointPositionsStreamed): %w", name, probeErr)
	default:
	}
	if probeErr != nil {
		cancel()
		ms.closeStream(st)
		return servoStarted{}, fmt.Errorf("servo_start: %w", probeErr)
	}
	abort := func(err error) (servoStarted, error) {
		cancel()
		ms.closeStream(st)
		return servoStarted{}, fmt.Errorf("servo_start: %w", err)
	}

	ramp := time.Duration(rampMS * float64(time.Millisecond))
	timeout := time.Duration(timeoutMS * float64(time.Millisecond))
	loop, err := servo.NewLoop(model, q0, func(c context.Context) ([]float64, error) { return su.a.JointPositions(c, nil) },
		solver, limits, st, time.Now, servo.LoopConfig{
			RateHz: rate, ResyncDeg: resyncDeg, Timeout: timeout, Ramp: ramp, Task: task, T0: t0, CloseTimeout: servoCloseWait,
		})
	if err != nil {
		return abort(err)
	}
	id := uuid.NewString()
	sess := servo.NewSession(id, loop, newResolver(su.fs, name), servo.SessionConfig{
		Mode: mode, MaxTargetStepMM: stepMM, MaxTargetStepDeg: stepDeg,
	})
	if err := ms.reg.publish(e, servoEntry{
		id: id, session: sess, cancel: cancel, done: e.done, arm: name, ramp: ramp, config: kinConfig, streamed: p.streamed,
		stopBudget: ramp + servoArmStopWait + servoCloseWait + time.Second,
	}); err != nil {
		return abort(err)
	}
	published = true
	go ms.runServo(e, loop, su.a, cancel)

	return servoStarted{entry: e, limits: limits, rateHz: rate, solver: solver.Name()}, nil
}

// probePoint holds the arm at q. Velocities and accelerations are zero-filled because a real arm client
// rejects trajectory points whose arrays don't match the arm's DoF.
func probePoint(q []float64) streaming.Point {
	return streaming.Point{Positions: q, Velocities: make([]float64, len(q)), Accelerations: make([]float64, len(q))}
}

func (ms *builtIn) closeStream(st *streaming.ArmStreamer) {
	if err := st.Close(); err != nil {
		ms.logger.Debugf("closing servo arm stream: %v", err)
	}
}

// moveOptions reads the optional limits object (degrees) into arm.MoveOptions (radians). Values are checked by validate.
func moveOptions(req map[string]interface{}) (arm.MoveOptions, error) {
	var o arm.MoveOptions
	lim, err := obj(req, "limits")
	if err != nil || lim == nil {
		return o, err
	}
	scalar := func(key string) (float64, error) { return num(lim, key, 0) }
	list := func(key string) ([]float64, error) {
		l, err := nums(lim, key)
		for i := range l {
			l[i] *= deg2rad
		}
		return l, err
	}
	var v, a float64
	if v, err = scalar("max_vel_degs_per_sec"); err != nil {
		return o, err
	}
	if a, err = scalar("max_acc_degs_per_sec2"); err != nil {
		return o, err
	}
	o.MaxVelRads, o.MaxAccRads = v*deg2rad, a*deg2rad
	if o.MaxVelRadsJoints, err = list("max_vel_degs_per_sec_joints"); err != nil {
		return o, err
	}
	if o.MaxAccRadsJoints, err = list("max_acc_degs_per_sec2_joints"); err != nil {
		return o, err
	}
	if lim["max_tcp_speed"] != nil {
		t, err := scalar("max_tcp_speed")
		if err != nil {
			return o, err
		}
		o.MaxTCPSpeedMPerSec = &t
	}
	return o, nil
}

// runServo is the session goroutine. Order matters: arm.Stop as soon as the loop is terminal and FAULTED
// (Run may linger up to CloseTimeout closing a hung stream), then wait for Run (which closes the streamer),
// release the registry, and close done last.
func (ms *builtIn) runServo(e *servoEntry, loop *servo.Loop, a arm.Arm, cancel context.CancelFunc) {
	defer close(e.done)
	defer ms.reg.release(e.arm, holderServo)
	defer cancel()
	ran := make(chan error, 1)
	go func() { ran <- loop.Run(context.Background()) }()
	<-loop.Done()
	final := loop.Status().State
	if final == servo.StateFaulted {
		sctx, c := context.WithTimeout(context.Background(), servoArmStopWait)
		if err := a.Stop(sctx, nil); err != nil {
			ms.logger.Warnf("servo session %s: arm.Stop after fault failed: %v", e.id, err)
		}
		c()
	}
	if err := <-ran; err != nil {
		ms.logger.Warnf("servo session %s on %q ended %s: %v", e.id, e.arm, final, err)
	}
}

// newResolver builds the session's Resolve from the frame system captured at start. Frames other than the
// arm's (and static ones) are held at zero inputs, so a target frame whose chain includes another moving
// component is refused.
func newResolver(fs *referenceframe.FrameSystem, arm string) servo.Resolve {
	base := arm + "_origin" // the arm model's base frame: where the loop's FK poses live
	zero := referenceframe.NewZeroInputs(fs)
	var mu sync.Mutex
	chains := map[string]error{}
	check := func(frame string) error {
		mu.Lock()
		defer mu.Unlock()
		if err, ok := chains[frame]; ok {
			return err
		}
		err := func() error {
			chain, err := fs.TracebackFrame(fs.Frame(frame))
			if err != nil {
				return fmt.Errorf("frame %q: %w", frame, err)
			}
			for _, c := range chain {
				if c.Name() != arm && len(c.DoF()) > 0 {
					return fmt.Errorf("frame %q is attached to movable frame %q (only the session's arm is supported)", frame, c.Name())
				}
			}
			return nil
		}()
		chains[frame] = err
		return err
	}
	return func(t *referenceframe.PoseInFrame, q []float64) (spatialmath.Pose, error) {
		if t.Parent() == "" || t.Parent() == base {
			return t.Pose(), nil
		}
		if fs.Frame(t.Parent()) == nil { // before the cache, so client-chosen names can't grow it
			return nil, fmt.Errorf("unknown frame %q", t.Parent())
		}
		if err := check(t.Parent()); err != nil {
			return nil, err
		}
		in := make(referenceframe.FrameSystemInputs, len(zero))
		for k, v := range zero {
			in[k] = v
		}
		in[arm] = q
		tf, err := fs.Transform(in.ToLinearInputs(), t, base)
		if err != nil {
			return nil, fmt.Errorf("transforming target from %q: %w", t.Parent(), err)
		}
		return tf.(*referenceframe.PoseInFrame).Pose(), nil
	}
}

// stopServoForMove makes Move respect the arm registry: servo sessions on arms in req.ComponentName's chain are
// stopped and awaited, and arms held by a stream (or a servo start in progress) are refused. Caller holds ms.mu.RLock.
func (ms *builtIn) stopServoForMove(ctx context.Context, req motion.MoveReq) error {
	if !ms.reg.busy() {
		return nil // ordinary Moves don't pay for a frame system build
	}
	fs, err := ms.getFrameSystem(ctx, req.WorldState.Transforms())
	if err != nil {
		return err
	}
	chain, err := chainFrames(fs, req.ComponentName)
	if err != nil {
		return err
	}
	for _, f := range chain {
		h, ok := ms.reg.get(f.Name())
		if !ok || h.kind == holderTeleop {
			continue
		}
		if h.kind == holderStream {
			return fmt.Errorf("arm %q is held by an arm-streaming session", f.Name())
		}
		e, ok := ms.reg.getServo(f.Name())
		if !ok { // released since the get
			continue
		}
		if e.session == nil {
			return fmt.Errorf("arm %q is starting a servo session", f.Name())
		}
		e.session.BeginStop(servo.StateStopped)
		if err := waitEntry(ctx, &e); err != nil {
			return err
		}
	}
	return nil
}

// --- the other verbs ---

// findServo resolves session_id or component_name to a running session.
func (ms *builtIn) findServo(req map[string]interface{}) (*servoEntry, error) {
	id, _ := req["session_id"].(string)
	arm, _ := req["component_name"].(string)
	if id == "" && arm == "" {
		return nil, errors.New("session_id or component_name is required")
	}
	pick := func(a string, e *servoEntry) bool { return (id != "" && e.id == id) || (id == "" && a == arm) }
	for a, h := range ms.reg.all(holderServo) {
		if pick(a, h.servo) {
			return h.servo, nil
		}
	}
	for a, e := range ms.reg.ended() { // finished: status and stop still report its final state
		if pick(a, e) {
			return e, nil
		}
	}
	return nil, errors.New("no servo session matches")
}

func replyMap(r servo.Reply) map[string]interface{} {
	return map[string]interface{}{"state": string(r.State), "applied": r.Applied, "err_mm": r.ErrMM, "err_deg": r.ErrDeg}
}

func (ms *builtIn) servoTarget(_ context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	e, err := ms.findServo(req)
	if err != nil {
		return nil, fmt.Errorf("servo_target: %w", err)
	}
	if e.streamed {
		return nil, fmt.Errorf("servo_target: session %s is owned by a StreamServo call", e.id)
	}
	t, err := obj(req, "target")
	if err != nil || t == nil {
		return nil, errors.New("servo_target: target is required")
	}
	frame, err := frameOf(t)
	if err != nil {
		return nil, fmt.Errorf("servo_target: %w", err)
	}
	p, err := obj(t, "pose")
	if err != nil || p == nil {
		return nil, errors.New("servo_target: target.pose is required")
	}
	var c [7]float64
	for i, k := range []string{"x", "y", "z", "o_x", "o_y", "o_z", "theta"} {
		if c[i], err = num(p, k, 0); err != nil {
			return nil, fmt.Errorf("servo_target: pose.%w", err)
		}
	}
	if !(math.Abs(c[3])+math.Abs(c[4])+math.Abs(c[5]) > 0) || math.IsInf(c[3]+c[4]+c[5], 0) {
		return nil, errors.New("servo_target: pose orientation vector (o_x, o_y, o_z) must be finite and non-zero")
	}
	ov := &spatialmath.OrientationVectorDegrees{OX: c[3], OY: c[4], OZ: c[5], Theta: c[6]}
	ov.Normalize()
	seq, err := seqOf(req)
	if err != nil {
		return nil, fmt.Errorf("servo_target: %w", err)
	}
	r, err := e.session.Target(referenceframe.NewPoseInFrame(frame, spatialmath.NewPose(r3.Vector{X: c[0], Y: c[1], Z: c[2]}, ov)), seq, nil)
	if err != nil {
		return nil, fmt.Errorf("servo_target: %w", err)
	}
	return replyMap(r), nil
}

func (ms *builtIn) servoVelocity(_ context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	e, err := ms.findServo(req)
	if err != nil {
		return nil, fmt.Errorf("servo_velocity: %w", err)
	}
	if e.streamed {
		return nil, fmt.Errorf("servo_velocity: session %s is owned by a StreamServo call", e.id)
	}
	lin, err := vec(req, "linear_mm_per_sec", 1e-3)
	if err != nil {
		return nil, fmt.Errorf("servo_velocity: %w", err)
	}
	ang, err := vec(req, "angular_degs_per_sec", deg2rad)
	if err != nil {
		return nil, fmt.Errorf("servo_velocity: %w", err)
	}
	frame, err := frameOf(req)
	if err != nil {
		return nil, fmt.Errorf("servo_velocity: %w", err)
	}
	seq, err := seqOf(req)
	if err != nil {
		return nil, fmt.Errorf("servo_velocity: %w", err)
	}
	r, err := e.session.Velocity(lin, ang, frame, seq)
	if err != nil {
		return nil, fmt.Errorf("servo_velocity: %w", err)
	}
	return map[string]interface{}{"state": string(r.State), "applied": r.Applied}, nil
}

func (ms *builtIn) servoStop(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	e, err := ms.findServo(req)
	if err != nil {
		return nil, fmt.Errorf("servo_stop: %w", err)
	}
	e.session.BeginStop(servo.StateStopped)
	if err := waitEntry(ctx, e); err != nil {
		return nil, fmt.Errorf("servo_stop: %w", err)
	}
	return map[string]interface{}{"state": string(e.session.Status().State)}, nil
}

func (ms *builtIn) servoStatus(_ context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	e, err := ms.findServo(req)
	if err != nil {
		return nil, fmt.Errorf("servo_status: %w", err)
	}
	s := e.session.Status()
	limitedBy := make([]string, len(s.LimitedBy))
	for i, l := range s.LimitedBy {
		limitedBy[i] = string(l)
	}
	ms64 := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return map[string]interface{}{
		"state": string(s.State), "mode": string(s.Mode), "last_seq": s.LastSeq, "err_mm": s.ErrMM, "err_deg": s.ErrDeg,
		"q_cmd_deg": toDeg(s.QCmd), "q_meas_deg": toDeg(s.QMeas), "ticks": s.Ticks, "overruns": s.Overruns,
		"solver_failures": s.SolverFailures, "resyncs": s.Resyncs, "limited": s.Limited, "damping": s.Damping,
		"last_tick_ms": ms64(s.LastTick), "tick_p99_ms": ms64(s.TickP99), "limits": limitsReply(s.Limits),
		"limited_by": limitedBy,
	}, nil
}
