package servo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// Mode is a session's command mode, fixed for its life.
type Mode string

// Modes.
const (
	ModePose     Mode = "pose"
	ModeVelocity Mode = "velocity"
)

var errStopping = errors.New("session stopping")

func finite(vs ...float64) bool {
	return !slices.ContainsFunc(vs, func(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) })
}

const (
	defaultMaxStepMM  = 100.0
	defaultMaxStepDeg = 30.0
)

// SessionConfig configures a Session. Zero step bounds take the defaults.
type SessionConfig struct {
	Mode             Mode
	MaxTargetStepMM  float64 // default 100
	MaxTargetStepDeg float64 // default 30
}

// Resolve turns a target in any frame into the arm's base frame using the arm's joints q; the glue
// supplies it from the cached frame system. It errors for frames on other moving components.
type Resolve func(target *referenceframe.PoseInFrame, q []float64) (spatialmath.Pose, error)

// Reply is what a command returns.
type Reply struct {
	State         State
	Applied       bool // false: dropped as out of order (seq)
	ErrMM, ErrDeg float64
}

// SessionStatus is the loop's status plus the session's contract state.
type SessionStatus struct {
	Status
	Mode    Mode
	LastSeq int64 // highest applied seq; 0 until a command carries one
}

// Session is the client-facing contract around a Loop.
type Session struct {
	id        string
	loop      *Loop
	resolve   Resolve
	cfg       SessionConfig
	maxStepMM float64
	maxStepDg float64

	mu      sync.Mutex
	lastSeq int64
	lastRaw spatialmath.Pose // base frame: the last applied target, or the start pose
}

// NewSession wraps loop (which the caller runs) in the contract. The first target step bound is measured
// from the loop's initial target (the arm's start pose).
func NewSession(id string, loop *Loop, resolve Resolve, cfg SessionConfig) *Session {
	s := &Session{
		id: id, loop: loop, resolve: resolve, cfg: cfg, lastRaw: loop.initialTarget(),
		maxStepMM: cfg.MaxTargetStepMM, maxStepDg: cfg.MaxTargetStepDeg,
	}
	if s.maxStepMM == 0 {
		s.maxStepMM = defaultMaxStepMM
	}
	if s.maxStepDg == 0 {
		s.maxStepDg = defaultMaxStepDeg
	}
	return s
}

func (s *Session) reply(applied bool) Reply {
	st := s.loop.Status()
	return Reply{State: st.State, Applied: applied, ErrMM: st.ErrMM, ErrDeg: st.ErrDeg}
}

// usable errors for a wrong mode, a finished loop or a stop under way. Caller holds s.mu.
func (s *Session) usable(m Mode) error {
	select {
	case <-s.loop.Done():
		return fmt.Errorf("session ended: %s", s.loop.Status().State)
	default:
	}
	if s.cfg.Mode != m {
		return fmt.Errorf("wrong mode: session is %s", s.cfg.Mode)
	}
	if s.loop.isStopping() {
		return errStopping
	}
	return nil
}

// checkSeq reports whether a command with seq applies: a seq at or below the last applied one is dropped
// (false, nil); seq ≤ 0 is an error, and so is no seq once the session has used one. Caller holds s.mu.
func (s *Session) checkSeq(seq *int64) (bool, error) {
	switch {
	case seq == nil && s.lastSeq > 0:
		return false, errors.New("seq is required once a session has used seq")
	case seq == nil:
		return true, nil
	case *seq <= 0:
		return false, fmt.Errorf("seq must be > 0, got %d", *seq)
	}
	return *seq > s.lastSeq, nil
}

// q is the arm's latest measured joints, or the start joints before the first tick.
func (s *Session) q() []float64 {
	if q := s.loop.Status().QMeas; len(q) > 0 {
		return q
	}
	return slices.Clone(s.loop.q0)
}

// checkStep rejects a base-frame target farther than the step bounds from the last applied one. Pointing
// counts only the tool-axis angle. Caller holds s.mu.
func (s *Session) checkStep(p spatialmath.Pose) error {
	q, q0 := p.Orientation().Quaternion(), s.lastRaw.Orientation().Quaternion()
	var deg float64
	if s.loop.cfg.Task == TaskPointing {
		deg = math.Acos(math.Max(-1, math.Min(1, toolAxis(q).Dot(toolAxis(q0))))) * 180 / math.Pi
	} else {
		deg = rotVec(q, q0).Norm() * 180 / math.Pi
	}
	mm := p.Point().Sub(s.lastRaw.Point()).Norm()
	if mm > s.maxStepMM || deg > s.maxStepDg {
		return fmt.Errorf("target is %.1f mm / %.1f deg from the last applied target (max_target_step_mm %v, max_target_step_deg %v)",
			mm, deg, s.maxStepMM, s.maxStepDg)
	}
	return nil
}

// FeedForward is a target's own velocity: Lin in m/s, Ang in rad/s, expressed in Frame. The frame is
// resolved through the frame system; callers pass "world" for an empty frame.
type FeedForward struct {
	Lin, Ang r3.Vector
	Frame    string
}

// Target sets the pose target (pose mode), moving on at ff (nil is zero) until the loop's hold ends.
// A rejected or stale target changes nothing.
func (s *Session) Target(t *referenceframe.PoseInFrame, seq *int64, ff *FeedForward) (Reply, error) {
	if t == nil {
		return Reply{}, errors.New("target is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.usable(ModePose); err != nil {
		return Reply{}, err
	}
	ok, err := s.checkSeq(seq)
	if err != nil {
		return Reply{}, err
	}
	if !ok {
		return s.reply(false), nil
	}
	p, err := s.resolve(t, s.q())
	if err != nil {
		return Reply{}, err
	}
	pt, o := p.Point(), p.Orientation().Quaternion()
	if !finite(pt.X, pt.Y, pt.Z, o.Real, o.Imag, o.Jmag, o.Kmag) {
		return Reply{}, errors.New("target is not finite")
	}
	if err := s.checkStep(p); err != nil {
		return Reply{}, err
	}
	var ff6 [6]float64
	if ff != nil {
		if ff6, err = s.twistToBase(ff.Lin, ff.Ang, ff.Frame); err != nil {
			return Reply{}, fmt.Errorf("feed-forward: %w", err)
		}
	}
	if !s.loop.SetTarget(p, ff6) {
		return Reply{}, errStopping
	}
	if seq != nil {
		s.lastSeq = *seq
	}
	s.lastRaw = p
	return s.reply(true), nil
}

// Velocity sets a twist (m/s, rad/s) in frame; it expires unless resent. The frame is resolved through the
// frame system; callers pass "world" for an empty frame (an empty frame here is the base frame).
func (s *Session) Velocity(lin, ang r3.Vector, frame string, seq *int64) (Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.usable(ModeVelocity); err != nil {
		return Reply{}, err
	}
	ok, err := s.checkSeq(seq)
	if err != nil {
		return Reply{}, err
	}
	if !ok {
		return s.reply(false), nil
	}
	v, err := s.twistToBase(lin, ang, frame)
	if err != nil {
		return Reply{}, err
	}
	if !s.loop.SetVelocity(v) {
		return Reply{}, errStopping
	}
	if seq != nil {
		s.lastSeq = *seq
	}
	return s.reply(true), nil
}

// BeginStop starts a ramp to zero ending in final. Idempotent; never blocks.
func (s *Session) BeginStop(final State) {
	s.loop.BeginStop(final)
}

// Stopping reports whether a stop ramp has begun (or the loop has ended).
func (s *Session) Stopping() bool {
	select {
	case <-s.loop.Done():
		return true
	default:
		return s.loop.isStopping()
	}
}

// Done is closed when the control loop is terminal (not when the glue's cleanup has run).
func (s *Session) Done() <-chan struct{} { return s.loop.Done() }

// Stop begins a stop and waits for the loop to finish or ctx to end.
func (s *Session) Stop(ctx context.Context, final State) (Reply, error) {
	s.BeginStop(final)
	select {
	case <-s.loop.Done():
		return s.reply(false), nil
	case <-ctx.Done():
		return Reply{}, ctx.Err()
	}
}

// Status is the loop's status plus the mode and last applied seq.
func (s *Session) Status() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionStatus{Status: s.loop.Status(), Mode: s.cfg.Mode, LastSeq: s.lastSeq}
}

// twistToBase validates a twist (finite, within the limits) and expresses it in the base frame. The frame is
// resolved through the frame system; callers pass "world" for an empty frame, and "" is the base frame.
// Caller holds s.mu.
func (s *Session) twistToBase(lin, ang r3.Vector, frame string) ([6]float64, error) {
	if !finite(lin.X, lin.Y, lin.Z, ang.X, ang.Y, ang.Z) {
		return [6]float64{}, errors.New("velocity is not finite")
	}
	l := s.loop.Status().Limits
	if n := lin.Norm(); n > l.MaxLinear {
		return [6]float64{}, fmt.Errorf("linear %.3g m/s exceeds max %.3g m/s", n, l.MaxLinear)
	}
	if n := ang.Norm(); n > l.MaxAngular {
		return [6]float64{}, fmt.Errorf("angular %.3g rad/s exceeds max %.3g rad/s", n, l.MaxAngular)
	}
	if frame != "" { // rotate by the frame's orientation in the base frame
		p, err := s.resolve(referenceframe.NewPoseInFrame(frame, spatialmath.NewZeroPose()), s.q())
		if err != nil {
			return [6]float64{}, err
		}
		q := p.Orientation().Quaternion()
		lin, ang = rotate(q, lin), rotate(q, ang)
	}
	return [6]float64{lin.X, lin.Y, lin.Z, ang.X, ang.Y, ang.Z}, nil
}

// Err is the cause of a FAULTED end; nil otherwise.
func (s *Session) Err() error { return s.loop.Err() }
