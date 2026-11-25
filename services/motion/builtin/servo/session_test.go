package servo

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// sessRig is a Session over a really running loop (real clock) on a UR5e with a perfect arm.
type sessRig struct {
	s     *Session
	l     *Loop
	out   *fakeStreamer
	start spatialmath.Pose
	mu    sync.Mutex
	qs    [][]float64 // the joints Resolve was called with
}

// baseRot is the orientation of the "rot" frame in the base frame (90° about z).
var baseRot = spatialmath.Quaternion(rotZ(1.5707963267948966))

func newSessRig(t *testing.T, mode Mode, task Task, mutate func(*SessionConfig, *LoopConfig)) *sessRig {
	t.Helper()
	return buildSessRig(t, mode, task, mutate, nil, true)
}

// buildSessRig builds the rig; fixedRead, if set, replaces the perfect-arm read; run false leaves the loop unstarted.
func buildSessRig(t *testing.T, mode Mode, task Task, mutate func(*SessionConfig, *LoopConfig), fixedRead ReadJoints, run bool) *sessRig {
	t.Helper()
	out := newFakeStreamer()
	model := loadModel(t, "ur5e")
	loopCfg := LoopConfig{RateHz: 200, Ramp: 20 * time.Millisecond, Timeout: 200 * time.Millisecond, Task: task}
	cfg := SessionConfig{Mode: mode}
	if mutate != nil {
		mutate(&cfg, &loopCfg)
	}
	read := func(context.Context) ([]float64, error) { // a perfect arm: the last commanded joints
		if p := out.points(); len(p) > 0 {
			return p[len(p)-1].Positions, nil
		}
		return ur5eHome, nil
	}
	lim := testLimits(t, model)
	lim.TightenLinear(0.1) // 100 mm/s
	if fixedRead != nil {
		read = fixedRead
	}
	l, err := NewLoop(model, ur5eHome, read, newDLSSolver(), lim, out, time.Now, loopCfg)
	test.That(t, err, test.ShouldBeNil)
	r := &sessRig{l: l, out: out, start: offset(t, model, r3.Vector{})}
	test.That(t, spatialmath.PoseAlmostEqual(l.initialTarget(), r.start), test.ShouldBeTrue)
	resolve := func(p *referenceframe.PoseInFrame, q []float64) (spatialmath.Pose, error) {
		r.mu.Lock()
		r.qs = append(r.qs, q)
		r.mu.Unlock()
		if p.Parent() == "rot" {
			return spatialmath.NewPose(p.Pose().Point(), &baseRot), nil
		}
		return p.Pose(), nil
	}
	r.s = NewSession("s1", l, resolve, cfg)
	if !run {
		return r
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = l.Run(ctx) }()
	t.Cleanup(func() {
		r.s.BeginStop(StateStopped)
		select {
		case <-l.Done():
		case <-time.After(2 * time.Second):
		}
		cancel()
		<-done
	})
	return r
}

type loopSnap struct {
	target     spatialmath.Pose
	ff         [6]float64
	lastUpdate time.Time
}

func (r *sessRig) snap() loopSnap {
	r.l.mu.Lock()
	defer r.l.mu.Unlock()
	return loopSnap{r.l.target, r.l.ff, r.l.lastUpdate}
}

func (r *sessRig) tgt(d r3.Vector) *referenceframe.PoseInFrame {
	return referenceframe.NewPoseInFrame("base", spatialmath.NewPose(r.start.Point().Add(d), r.start.Orientation()))
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func i64(v int64) *int64 { return &v }

func errHas(t *testing.T, err error, substr string) {
	t.Helper()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, substr)
}

// An update applies only above the last applied seq; a dropped one changes nothing (target, twist,
// dead-man); once a session has used seq, an update without one is an error. Without seq, all apply.
func TestSessionSeq(t *testing.T) {
	type step struct {
		seq     *int64
		applied bool
		err     string
	}
	for _, c := range []struct {
		name  string
		steps []step
	}{
		{"no seq always applies", []step{{nil, true, ""}, {nil, true, ""}}},
		{"increasing", []step{{i64(1), true, ""}, {i64(2), true, ""}, {i64(10), true, ""}}},
		{"repeat and older dropped", []step{{i64(5), true, ""}, {i64(5), false, ""}, {i64(4), false, ""}, {i64(6), true, ""}}},
		{"seq-less after seq rejected", []step{{nil, true, ""}, {i64(1), true, ""}, {nil, false, "seq is required"}}},
		{"zero rejected", []step{{i64(0), false, "seq must be > 0"}}},
		{"negative rejected", []step{{i64(3), true, ""}, {i64(-1), false, "seq must be > 0"}}},
	} {
		for _, mode := range []Mode{ModePose, ModeVelocity} {
			t.Run(c.name+"/"+string(mode), func(t *testing.T) {
				r := newSessRig(t, mode, TaskPose, nil)
				for i, st := range c.steps {
					before := r.snap()
					d := float64(i + 1)
					var rep Reply
					var err error
					if mode == ModePose {
						rep, err = r.s.Target(r.tgt(r3.Vector{X: d}), st.seq, nil)
					} else {
						rep, err = r.s.Velocity(r3.Vector{X: 0.001 * d}, r3.Vector{}, "", st.seq)
					}
					if st.err != "" {
						errHas(t, err, st.err)
					} else {
						test.That(t, err, test.ShouldBeNil)
					}
					after := r.snap()
					changed := after.lastUpdate != before.lastUpdate
					if mode == ModePose {
						changed = changed && !spatialmath.PoseAlmostEqual(after.target, before.target)
					} else {
						changed = changed && after.ff != before.ff
					}
					test.That(t, rep.Applied, test.ShouldEqual, st.applied)
					test.That(t, changed, test.ShouldEqual, st.applied)
				}
			})
		}
	}
}

// A target farther than the step bounds from the last applied one (the start pose for the first) is
// rejected: nothing changes and the seq is not used up.
func TestSessionTargetStepBound(t *testing.T) {
	for _, c := range []struct {
		name          string
		task          Task
		stepMM        float64 // 0: default 100
		d             r3.Vector
		rotDeg        float64 // about base z, or about the tool axis for roll
		roll, wantErr bool
	}{
		{name: "99 mm", d: r3.Vector{X: 99}},
		{name: "101 mm", d: r3.Vector{X: 101}, wantErr: true},
		{name: "29 deg", rotDeg: 29},
		{name: "31 deg", rotDeg: 31, wantErr: true},
		{name: "150 mm under a 200 mm bound", stepMM: 200, d: r3.Vector{Y: 150}},
		{name: "pose task counts roll", roll: true, rotDeg: 90, wantErr: true},
		{name: "pointing task ignores roll", task: TaskPointing, roll: true, rotDeg: 90},
		{name: "pointing task counts the axis", task: TaskPointing, rotDeg: 31, wantErr: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newSessRig(t, ModePose, c.task, func(cfg *SessionConfig, _ *LoopConfig) { cfg.MaxTargetStepMM = c.stepMM })
			q := r.start.Orientation().Quaternion()
			axis := r3.Vector{Z: 1}
			if c.roll {
				axis = toolAxis(q)
			}
			if c.task == TaskPointing && !c.roll {
				axis = toolAxis(q).Cross(r3.Vector{Z: 1}).Normalize() // tilts the tool axis
			}
			rq := spatialmath.Quaternion(quat.Mul(expRot(axis.Mul(c.rotDeg*3.141592653589793/180)), q))
			far := referenceframe.NewPoseInFrame("base", spatialmath.NewPose(r.start.Point().Add(c.d), &rq))
			before := r.snap()
			rep, err := r.s.Target(far, i64(1), nil)
			if !c.wantErr {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, rep.Applied, test.ShouldBeTrue)
				return
			}
			errHas(t, err, "from the last applied target")
			test.That(t, r.snap(), test.ShouldResemble, before)
			test.That(t, r.s.Status().LastSeq, test.ShouldEqual, 0)
			// the seq wasn't used up, and the bound is from the last applied target
			rep, err = r.s.Target(r.tgt(r3.Vector{Z: 60}), i64(1), nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rep.Applied, test.ShouldBeTrue)
			_, err = r.s.Target(r.tgt(r3.Vector{Z: 140}), i64(2), nil) // 140 from start, 80 from the last
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func TestSessionWrongModeAndVelocityLimits(t *testing.T) {
	p := newSessRig(t, ModePose, TaskPose, nil)
	_, err := p.s.Velocity(r3.Vector{}, r3.Vector{}, "", nil)
	errHas(t, err, "wrong mode: session is pose")

	v := newSessRig(t, ModeVelocity, TaskPose, nil)
	_, err = v.s.Target(p.tgt(r3.Vector{}), nil, nil)
	errHas(t, err, "wrong mode: session is velocity")
	before := v.snap()
	_, err = v.s.Velocity(r3.Vector{X: 100}, r3.Vector{}, "", nil)
	errHas(t, err, "linear 100 m/s exceeds max 0.1 m/s")
	_, err = v.s.Velocity(r3.Vector{}, r3.Vector{Z: 100}, "", nil)
	errHas(t, err, "angular 100 rad/s exceeds max")
	test.That(t, v.snap(), test.ShouldResemble, before)
}

func TestSessionVelocityFrame(t *testing.T) {
	r := newSessRig(t, ModeVelocity, TaskPose, nil)
	rep, err := r.s.Velocity(r3.Vector{X: 0.01}, r3.Vector{Z: 0.1}, "rot", nil) // x of a frame turned 90° about z is base y
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rep.Applied, test.ShouldBeTrue)
	ff := r.snap().ff
	nearVec(t, ff[:], []float64{0, 0.01, 0, 0, 0, 0.1}, 1e-9)
	_, err = r.s.Velocity(r3.Vector{X: 0.01}, r3.Vector{}, "", nil) // "" is the base frame
	test.That(t, err, test.ShouldBeNil)
	ff = r.snap().ff
	nearVec(t, ff[:], []float64{0.01, 0, 0, 0, 0, 0}, 1e-12)
}

func TestSessionResolveGetsMeasuredJoints(t *testing.T) {
	off := make([]float64, len(ur5eHome))
	for i, v := range ur5eHome {
		off[i] = v + 0.01
	}
	fixed := func(context.Context) ([]float64, error) { return off, nil }
	// before any tick: the start joints
	r := buildSessRig(t, ModePose, TaskPose, nil, fixed, false)
	_, err := r.s.Target(r.tgt(r3.Vector{X: 1}), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	r.mu.Lock()
	nearVec(t, r.qs[0], ur5eHome, 1e-12)
	r.mu.Unlock()
	// after a tick: the measured joints
	r = newSessRigRead(t, fixed)
	eventually(t, "a tick", func() bool { return r.l.Status().Ticks > 0 })
	_, err = r.s.Target(r.tgt(r3.Vector{X: 1}), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	r.mu.Lock()
	defer r.mu.Unlock()
	nearVec(t, r.qs[0], off, 1e-12)
}

func newSessRigRead(t *testing.T, read ReadJoints) *sessRig {
	t.Helper()
	return buildSessRig(t, ModePose, TaskPose, nil, read, true)
}

func TestSessionRejectsNonFinite(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	p := newSessRig(t, ModePose, TaskPose, nil)
	before := p.snap()
	bad := referenceframe.NewPoseInFrame("base", spatialmath.NewPose(r3.Vector{X: nan}, p.start.Orientation()))
	_, err := p.s.Target(bad, i64(1), nil)
	errHas(t, err, "not finite")
	badQ := spatialmath.Quaternion(quat.Number{Real: inf})
	bad = referenceframe.NewPoseInFrame("base", spatialmath.NewPose(p.start.Point(), &badQ))
	_, err = p.s.Target(bad, i64(1), nil)
	errHas(t, err, "not finite")
	test.That(t, p.snap(), test.ShouldResemble, before)
	test.That(t, p.s.Status().LastSeq, test.ShouldEqual, 0)

	v := newSessRig(t, ModeVelocity, TaskPose, nil)
	before = v.snap()
	_, err = v.s.Velocity(r3.Vector{X: nan}, r3.Vector{}, "", i64(1))
	errHas(t, err, "not finite")
	_, err = v.s.Velocity(r3.Vector{}, r3.Vector{Z: nan}, "", i64(1))
	errHas(t, err, "not finite")
	test.That(t, v.snap(), test.ShouldResemble, before)
	test.That(t, v.s.Status().LastSeq, test.ShouldEqual, 0)
}

// A command that loses the race with a stop (the loop refuses it) is an error and uses up nothing.
func TestSessionLostStopRaceCommitsNothing(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	r.l.BeginStop(StateStopped) // behind the session's back, as the dead-man does
	_, err := r.s.Target(r.tgt(r3.Vector{X: 5}), i64(1), nil)
	errHas(t, err, "session stopping")
	test.That(t, r.s.Status().LastSeq, test.ShouldEqual, 0)
	test.That(t, r.l.SetTarget(r.start, [6]float64{}), test.ShouldBeFalse)
	test.That(t, r.l.SetVelocity([6]float64{}), test.ShouldBeFalse)
}

func TestSessionTargetConvergesAndReplies(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	rep, err := r.s.Target(r.tgt(r3.Vector{X: 20}), i64(1), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rep.Applied, test.ShouldBeTrue)
	var last Reply
	seq := int64(1)
	eventually(t, "convergence", func() bool {
		seq++
		last, err = r.s.Target(r.tgt(r3.Vector{X: 20}), &seq, nil) // resend to feed the dead-man
		test.That(t, err, test.ShouldBeNil)
		st := r.s.Status()
		return st.State == StateConverged && st.ErrMM < 2
	})
	test.That(t, last.Applied, test.ShouldBeTrue)
	st := r.s.Status()
	test.That(t, st.Mode, test.ShouldEqual, ModePose)
	test.That(t, st.LastSeq, test.ShouldEqual, seq)
	// a stale seq still replies with the current state
	rep, err = r.s.Target(r.tgt(r3.Vector{X: 20}), i64(1), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rep.Applied, test.ShouldBeFalse)
	test.That(t, rep.State, test.ShouldEqual, StateConverged)
}

func TestSessionStopAndCallsAfterTerminal(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	_, err := r.s.Target(r.tgt(r3.Vector{X: 5}), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rep, err := r.s.Stop(ctx, StateStopped)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rep.State, test.ShouldEqual, StateStopped)
	select {
	case <-r.s.Done():
	default:
		t.Fatal("Done not closed after Stop")
	}
	_, err = r.s.Target(r.tgt(r3.Vector{}), nil, nil)
	errHas(t, err, "session ended: stopped")
	r.s.BeginStop(StateStopped) // idempotent after the end
	_, err = r.s.Stop(ctx, StateStopped)
	test.That(t, err, test.ShouldBeNil)
}

func TestSessionRejectsCommandsWhileStopping(t *testing.T) {
	r := newSessRig(t, ModeVelocity, TaskPose, func(_ *SessionConfig, c *LoopConfig) { c.Ramp = time.Second })
	_, err := r.s.Velocity(r3.Vector{X: 0.01}, r3.Vector{}, "", nil)
	test.That(t, err, test.ShouldBeNil)
	r.s.BeginStop(StateStopped)
	before := r.snap()
	_, err = r.s.Velocity(r3.Vector{X: 0.02}, r3.Vector{}, "", nil)
	errHas(t, err, "session stopping")
	test.That(t, r.snap(), test.ShouldResemble, before)
}

func TestSessionDeadman(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	_, err := r.s.Target(r.tgt(r3.Vector{X: 5}), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	eventually(t, "dead-man stop", func() bool {
		select {
		case <-r.s.Done():
			return true
		default:
			return false
		}
	})
	test.That(t, r.s.Status().State, test.ShouldEqual, StateStoppedTimeout)
	_, err = r.s.Target(r.tgt(r3.Vector{}), nil, nil)
	errHas(t, err, "session ended: stopped_timeout")
}

func TestTargetFeedForwardMovesTarget(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	_, err := r.s.Target(r.tgt(r3.Vector{}), nil, &FeedForward{Lin: r3.Vector{X: 0.05}, Frame: "base"})
	test.That(t, err, test.ShouldBeNil)
	dx := func() float64 { return (r.snap().target.Point().X - r.start.Point().X) }
	eventually(t, "target moved ~5 mm", func() bool { return math.Abs(dx()-5) < 1.5 })
	eventually(t, "target at rest", func() bool {
		a := dx()
		time.Sleep(100 * time.Millisecond)
		return math.Abs(dx()-a) < 0.2
	})
	test.That(t, math.Abs(dx()-5), test.ShouldBeLessThan, 1.5)

	// no feed-forward: the target stays put
	r = newSessRig(t, ModePose, TaskPose, nil)
	_, err = r.s.Target(r.tgt(r3.Vector{}), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(250 * time.Millisecond)
	test.That(t, math.Abs(r.snap().target.Point().X-r.start.Point().X), test.ShouldBeLessThan, 0.2)
}

func TestTargetFeedForwardOverLimitRejected(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	before := r.snap()
	_, err := r.s.Target(r.tgt(r3.Vector{X: 5}), i64(1), &FeedForward{Lin: r3.Vector{X: 100}, Frame: "base"})
	errHas(t, err, "linear 100 m/s exceeds max")
	_, err = r.s.Target(r.tgt(r3.Vector{X: 5}), i64(1), &FeedForward{Ang: r3.Vector{Z: 100}, Frame: "base"})
	errHas(t, err, "angular 100 rad/s exceeds max")
	test.That(t, r.snap(), test.ShouldResemble, before)
	test.That(t, r.s.Status().LastSeq, test.ShouldEqual, 0)
}

func TestTargetFeedForwardNotFinite(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	before := r.snap()
	_, err := r.s.Target(r.tgt(r3.Vector{X: 5}), nil, &FeedForward{Lin: r3.Vector{X: math.NaN()}, Frame: "base"})
	errHas(t, err, "not finite")
	test.That(t, r.snap(), test.ShouldResemble, before)
}

func TestSessionErr(t *testing.T) {
	r := newSessRig(t, ModePose, TaskPose, nil)
	_, err := r.s.Target(r.tgt(r3.Vector{}), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	r.out.End(errors.New("stream broke"))
	eventually(t, "fault", func() bool {
		select {
		case <-r.s.Done():
			return true
		default:
			return false
		}
	})
	test.That(t, r.s.Status().State, test.ShouldEqual, StateFaulted)
	test.That(t, r.s.Err(), test.ShouldNotBeNil)

	r = newSessRig(t, ModePose, TaskPose, nil)
	_, err = r.s.Stop(context.Background(), StateStopped)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.s.Err(), test.ShouldBeNil)
}
