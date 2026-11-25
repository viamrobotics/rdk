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

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const testDT = 20 * time.Millisecond

var ur5eHome = []float64{0, -1.2, 1.5, -1.8, -1.57, 0}

// rig is a loop on a UR5e (pose task) with a perfect arm unless read is overridden.
type rig struct {
	l     *Loop
	clk   *fakeClock
	out   *fakeStreamer
	model referenceframe.Model
	lim   Limits
}

type rigOpts struct {
	solver Solver
	cfg    func(*LoopConfig)
	read   func(r *rig) ReadJoints
	lim    func(*Limits)
}

func newRig(t *testing.T, o rigOpts) *rig {
	t.Helper()
	r := &rig{clk: &fakeClock{t: time.Unix(1000, 0)}, out: newFakeStreamer(), model: loadModel(t, "ur5e")}
	r.lim = testLimits(t, r.model)
	if o.lim != nil {
		o.lim(&r.lim)
	}
	cfg := LoopConfig{Timeout: 10 * time.Second, Ramp: 200 * time.Millisecond, T0: r.clk.Now()}
	if o.cfg != nil {
		o.cfg(&cfg)
	}
	read := func(context.Context) ([]float64, error) { // a perfect arm: the last commanded joints
		if p := r.out.points(); len(p) > 0 {
			return p[len(p)-1].Positions, nil
		}
		return ur5eHome, nil
	}
	if o.read != nil {
		read = o.read(r)
	}
	if o.solver == nil {
		o.solver = newDLSSolver()
	}
	l, err := NewLoop(r.model, ur5eHome, read, o.solver, r.lim, r.out, r.clk.Now, cfg)
	test.That(t, err, test.ShouldBeNil)
	r.l = l
	return r
}

// tick advances the clock one period and ticks once.
func (r *rig) tick() error {
	r.clk.Advance(testDT)
	return r.l.Tick(context.Background())
}

func (r *rig) ticks(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		test.That(t, r.tick(), test.ShouldBeNil)
	}
}

// offset returns the home pose moved by d mm.
func offset(t *testing.T, m referenceframe.Model, d r3.Vector) spatialmath.Pose {
	t.Helper()
	p, q := mustFK(t, m, ur5eHome)
	qq := spatialmath.Quaternion(q)
	return spatialmath.NewPose(p.Mul(mmPerM).Add(d), &qq)
}

func targetX(l *Loop) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.target.Point().X
}

// failingSolver returns the scripted errors in order (nil: a zero step), then zero steps.
type scriptedSolver struct {
	errs  []error
	n     int
	onRun func()
	dq    func(n int) []float64
	info  StepInfo
}

func (s *scriptedSolver) Name() string { return "scripted" }

func (s *scriptedSolver) Step(in StepInput) ([]float64, StepInfo, error) {
	i := s.n
	s.n++
	if s.onRun != nil {
		s.onRun()
	}
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, StepInfo{}, s.errs[i]
	}
	if s.dq != nil {
		return s.dq(i), s.info, nil
	}
	return make([]float64, len(in.Q)), s.info, nil
}

func TestLoopTracksTarget(t *testing.T) {
	r := newRig(t, rigOpts{})
	r.l.SetTarget(offset(t, r.model, r3.Vector{X: 30}), [6]float64{})
	r.ticks(t, 300)
	s := r.l.Status()
	test.That(t, s.State, test.ShouldEqual, StateConverged)
	test.That(t, s.ErrMM, test.ShouldBeLessThan, 1)
	test.That(t, s.Ticks, test.ShouldEqual, 300)
	pts := r.out.points()
	test.That(t, len(pts), test.ShouldEqual, 300)
	for i := 1; i < len(pts); i++ {
		test.That(t, pts[i].Time, test.ShouldBeGreaterThan, pts[i-1].Time)
	}
}

func TestLoopDeadmanRampsToStoppedTimeout(t *testing.T) {
	r := newRig(t, rigOpts{cfg: func(c *LoopConfig) { c.Timeout = 500 * time.Millisecond }})
	r.l.SetTarget(offset(t, r.model, r3.Vector{X: 150}), [6]float64{})
	r.ticks(t, 20) // 400 ms: before the timeout
	test.That(t, r.l.Status().State, test.ShouldEqual, StateRunning)
	var err error
	for i := 0; i < 100 && err == nil; i++ {
		err = r.tick()
	}
	test.That(t, err, test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStoppedTimeout)
	select {
	case <-r.l.Done():
	default:
		t.Fatal("Done not closed")
	}
	pts := r.out.points()
	last := pts[len(pts)-1]
	for i, v := range last.Velocities {
		test.That(t, math.Abs(v)*testDT.Seconds(), test.ShouldBeLessThanOrEqualTo, r.lim.Acc[i]*testDT.Seconds()*testDT.Seconds()*(1+1e-9))
	}
	test.That(t, r.tick(), test.ShouldEqual, errDone)
	test.That(t, len(r.out.points()), test.ShouldEqual, len(pts))
}

// TestLoopRampReducesStepWhenCapped starts a stop far from the target with the linear cap active:
// the ramp scales dq, so the step must still fall to zero.
func TestLoopRampReducesStepWhenCapped(t *testing.T) {
	r := newRig(t, rigOpts{
		lim: func(l *Limits) { l.TightenLinear(0.05) },
		cfg: func(c *LoopConfig) { c.Ramp = 500 * time.Millisecond },
	})
	r.l.SetTarget(offset(t, r.model, r3.Vector{X: 300}), [6]float64{})
	r.ticks(t, 30)
	r.l.BeginStop(StateStopped)
	norm := func() float64 {
		p := r.out.points()
		s := 0.0
		for _, v := range p[len(p)-1].Velocities {
			s += v * v
		}
		return math.Sqrt(s)
	}
	prev := norm()
	test.That(t, prev, test.ShouldBeGreaterThan, 0)
	var err error
	for i := 0; i < 100 && err == nil; i++ {
		err = r.tick()
		if err == nil {
			n := norm()
			test.That(t, n, test.ShouldBeLessThanOrEqualTo, prev+1e-9)
			prev = n
		}
	}
	test.That(t, err, test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStopped)
	test.That(t, prev, test.ShouldBeLessThan, 1e-2)
}

func TestLoopVelocityExpires(t *testing.T) {
	r := newRig(t, rigOpts{})
	tw := [6]float64{0.05}
	advancing := func(n int) (moved int) {
		for i := 0; i < n; i++ {
			before := targetX(r.l)
			test.That(t, r.tick(), test.ShouldBeNil)
			if targetX(r.l)-before > 1e-9 {
				moved++
			}
		}
		return moved
	}
	r.l.SetVelocity(tw)
	// the first twist holds 200 ms: ticks at +20..+200 advance, then the target stops
	test.That(t, advancing(30), test.ShouldEqual, 10)
	x := targetX(r.l)
	r.ticks(t, 100)
	test.That(t, targetX(r.l), test.ShouldEqual, x)
	s := r.l.Status()
	test.That(t, s.State, test.ShouldEqual, StateConverged)
	pts := r.out.points()
	for _, v := range pts[len(pts)-1].Velocities {
		test.That(t, math.Abs(v), test.ShouldBeLessThan, 1e-3)
	}

	// a second twist 60 ms after the first holds min(2×60, 200) = 120 ms
	r2 := newRig(t, rigOpts{})
	r2.l.SetVelocity(tw)
	r2.ticks(t, 3)
	r2.l.SetVelocity(tw)
	moved := 0
	for i := 0; i < 20; i++ {
		before := targetX(r2.l)
		test.That(t, r2.tick(), test.ShouldBeNil)
		if targetX(r2.l)-before > 1e-9 {
			moved++
		}
	}
	test.That(t, moved, test.ShouldEqual, 6) // +80..+180 ms after the first twist
}

func TestLoopStreamEndFaults(t *testing.T) {
	r := newRig(t, rigOpts{})
	r.ticks(t, 3)
	r.out.End(errors.New("x"))
	test.That(t, r.tick(), test.ShouldBeNil) // inside the grace period
	test.That(t, r.l.Status().State, test.ShouldNotEqual, StateFaulted)
	r.clk.Advance(100 * time.Millisecond)
	test.That(t, r.l.Tick(context.Background()), test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateFaulted)
}

func TestLoopStreamEndAfterStopIsStopped(t *testing.T) {
	r := newRig(t, rigOpts{})
	r.ticks(t, 3)
	r.l.BeginStop(StateStopped)
	r.out.End(errors.New("x"))
	test.That(t, r.tick(), test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStopped)
}

func TestLoopStreamEndThenStopWithinGraceIsStopped(t *testing.T) {
	r := newRig(t, rigOpts{})
	r.ticks(t, 3)
	r.out.End(errors.New("x"))
	test.That(t, r.tick(), test.ShouldBeNil)
	r.clk.Advance(50 * time.Millisecond)
	r.l.BeginStop(StateStopped)
	test.That(t, r.l.Tick(context.Background()), test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStopped)
}

func TestLoopResync(t *testing.T) {
	lag := []float64{0.5, 0, 0, 0, 0, 0} // far more than ResyncDeg (3 ticks at 1 rad/s = 3.4°)
	r := newRig(t, rigOpts{read: func(*rig) ReadJoints {
		return func(context.Context) ([]float64, error) {
			q := append([]float64(nil), ur5eHome...)
			q[0] += lag[0]
			return q, nil
		}
	}})
	test.That(t, r.tick(), test.ShouldBeNil)
	s := r.l.Status()
	test.That(t, s.Resyncs, test.ShouldEqual, 1)
	test.That(t, math.Abs(s.QCmd[0]-s.QMeas[0]), test.ShouldBeLessThan, 1*testDT.Seconds()+1e-9)
}

func TestLoopSolverFailuresFault(t *testing.T) {
	boom := errors.New("boom")
	r := newRig(t, rigOpts{solver: &scriptedSolver{errs: []error{boom, boom, boom}}})
	test.That(t, r.tick(), test.ShouldBeNil)
	test.That(t, r.tick(), test.ShouldBeNil)
	test.That(t, r.tick(), test.ShouldEqual, errDone)
	s := r.l.Status()
	test.That(t, s.State, test.ShouldEqual, StateFaulted)
	test.That(t, s.SolverFailures, test.ShouldEqual, 3)

	test.That(t, len(r.out.points()), test.ShouldEqual, 3) // the braking point of the faulting tick is sent

	// failures between successes: q_cmd brakes (does not take the success step, nor hold literally)
	var errs []error
	for i := 0; i < 5; i++ {
		errs = append(errs, nil)
	}
	errs = append(errs, boom, nil, boom, nil)
	r = newRig(t, rigOpts{solver: &scriptedSolver{errs: errs, dq: func(int) []float64 { return []float64{0.05, 0, 0, 0, 0, 0} }}})
	r.ticks(t, 5)
	before, lastStep := r.l.Status().QCmd[0], r.l.lastStep[0]
	acc := r.lim.Acc[0] * testDT.Seconds() * testDT.Seconds()
	test.That(t, lastStep, test.ShouldBeGreaterThan, 2*acc)
	r.ticks(t, 1) // the failed tick
	test.That(t, r.l.Status().QCmd[0]-before, test.ShouldAlmostEqual, lastStep-acc, 1e-9)
	r.ticks(t, 3)
	s = r.l.Status()
	test.That(t, s.State, test.ShouldNotEqual, StateFaulted)
	test.That(t, s.SolverFailures, test.ShouldEqual, 2)
}

func TestLoopNonFiniteStepIsSolverFailure(t *testing.T) {
	bad := func(int) []float64 { return []float64{math.NaN(), 0, 0, 0, 0, 0} }
	r := newRig(t, rigOpts{solver: &scriptedSolver{dq: bad}})
	test.That(t, r.tick(), test.ShouldBeNil)
	test.That(t, r.tick(), test.ShouldBeNil)
	test.That(t, r.tick(), test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateFaulted)
	for _, p := range r.out.points() {
		for _, v := range append(append(p.Positions, p.Velocities...), p.Accelerations...) {
			test.That(t, math.IsNaN(v) || math.IsInf(v, 0), test.ShouldBeFalse)
		}
	}
}

func TestLoopOverrunCounted(t *testing.T) {
	var r *rig
	s := &scriptedSolver{}
	s.onRun = func() {
		if s.n == 1 {
			r.clk.Advance(30 * time.Millisecond)
		}
	}
	r = newRig(t, rigOpts{solver: s})
	r.ticks(t, 3)
	st := r.l.Status()
	test.That(t, st.Overruns, test.ShouldEqual, 1)
	test.That(t, st.LastTick, test.ShouldEqual, time.Duration(0))
	test.That(t, st.TickP99, test.ShouldEqual, 30*time.Millisecond)
}

func TestLoopStalledWhenDamped(t *testing.T) {
	r := newRig(t, rigOpts{solver: &scriptedSolver{info: StepInfo{Damping: 0.1}}})
	r.l.SetTarget(offset(t, r.model, r3.Vector{X: 100}), [6]float64{})
	r.ticks(t, 49)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateRunning)
	r.ticks(t, 5)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStalled)
}

func TestLoopRunStopsOnCancel(t *testing.T) {
	r := newRig(t, rigOpts{})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var runErr error
	wg.Add(1)
	go func() { defer wg.Done(); runErr = r.l.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for r.l.Status().Ticks < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	wg.Wait()
	test.That(t, runErr, test.ShouldEqual, context.Canceled)
	test.That(t, r.l.Status().Ticks, test.ShouldBeGreaterThanOrEqualTo, 3)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStopped)
	test.That(t, r.out.closed, test.ShouldEqual, 1)
	<-r.l.Done()
	test.That(t, r.l.Err(), test.ShouldBeNil) // a plain stop is not a fault
}

func TestLoopRunFaultsOnHungRead(t *testing.T) {
	r := newRig(t, rigOpts{
		cfg: func(c *LoopConfig) { c.TickTimeout = 30 * time.Millisecond },
		read: func(*rig) ReadJoints {
			return func(ctx context.Context) ([]float64, error) { <-ctx.Done(); return nil, ctx.Err() }
		},
	})
	err := r.l.Run(context.Background())
	test.That(t, errors.Is(err, context.DeadlineExceeded), test.ShouldBeTrue)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateFaulted)
}

// TestLoopRunDropsTicksAfterOverrun: a tick that overruns must not be followed by a catch-up tick.
func TestLoopRunDropsTicksAfterOverrun(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	var r *rig
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &scriptedSolver{}
	s.onRun = func() {
		mu.Lock()
		defer mu.Unlock()
		starts = append(starts, time.Now())
		switch len(starts) {
		case 1: // 55 ms of a 20 ms period; the fake clock sees it too
			time.Sleep(55 * time.Millisecond)
			r.clk.Advance(55 * time.Millisecond)
		case 6:
			cancel()
		}
	}
	r = newRig(t, rigOpts{solver: s})
	test.That(t, r.l.Run(ctx), test.ShouldEqual, context.Canceled)
	mu.Lock()
	defer mu.Unlock()
	test.That(t, len(starts), test.ShouldBeGreaterThanOrEqualTo, 6)
	for i := 2; i < len(starts); i++ { // a buffered catch-up tick would start ~5 ms before the next edge
		test.That(t, starts[i].Sub(starts[i-1]), test.ShouldBeGreaterThan, 12*time.Millisecond)
	}
	test.That(t, r.l.Status().Overruns, test.ShouldEqual, 1)
}

func TestLoopBeginStopFinalStates(t *testing.T) {
	for _, c := range []struct{ in, want State }{
		{StateStopped, StateStopped}, {StateStoppedTimeout, StateStoppedTimeout}, {StateFaulted, StateStopped}, {StateRunning, StateStopped},
	} {
		r := newRig(t, rigOpts{})
		r.ticks(t, 2)
		r.l.BeginStop(c.in)
		var err error
		for i := 0; i < 100 && err == nil; i++ {
			err = r.tick()
		}
		test.That(t, err, test.ShouldEqual, errDone)
		test.That(t, r.l.Status().State, test.ShouldEqual, c.want)
	}
}

func TestLoopBadReadFaults(t *testing.T) {
	for _, q := range map[string][]float64{
		"short": {0, 0},
		"nan":   {math.NaN(), 0, 0, 0, 0, 0},
		"inf":   {math.Inf(1), 0, 0, 0, 0, 0},
	} {
		r := newRig(t, rigOpts{read: func(*rig) ReadJoints { return func(context.Context) ([]float64, error) { return q, nil } }})
		test.That(t, r.tick(), test.ShouldEqual, errDone)
		test.That(t, r.l.Status().State, test.ShouldEqual, StateFaulted)
		test.That(t, len(r.out.points()), test.ShouldEqual, 0)
	}
}

func TestLoopSendErrorFaults(t *testing.T) {
	r := newRig(t, rigOpts{})
	r.ticks(t, 2)
	r.out.sendErr = errors.New("rpc broke")
	test.That(t, r.tick(), test.ShouldEqual, errDone)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateFaulted)
	test.That(t, r.l.cause, test.ShouldNotBeNil)
	test.That(t, r.l.Err(), test.ShouldNotBeNil)
}

func TestLoopRunBoundsClose(t *testing.T) {
	r := newRig(t, rigOpts{cfg: func(c *LoopConfig) { c.CloseTimeout, c.Ramp = 50*time.Millisecond, 0 }})
	r.out.block = make(chan struct{})
	defer close(r.out.block)
	r.l.BeginStop(StateStopped)
	begin := time.Now()
	err := r.l.Run(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "timed out")
	test.That(t, time.Since(begin), test.ShouldBeLessThan, 2*time.Second)
	test.That(t, r.l.Status().State, test.ShouldEqual, StateStopped)
}

func TestNewLoopRejectsNegativeTimeouts(t *testing.T) {
	for _, mut := range []func(*LoopConfig){
		func(c *LoopConfig) { c.TickTimeout = -1 }, func(c *LoopConfig) { c.CloseTimeout = -1 },
	} {
		clk := &fakeClock{t: time.Unix(1000, 0)}
		m := loadModel(t, "ur5e")
		cfg := LoopConfig{Timeout: time.Second}
		mut(&cfg)
		_, err := NewLoop(m, ur5eHome, func(context.Context) ([]float64, error) { return ur5eHome, nil }, newDLSSolver(),
			testLimits(t, m), newFakeStreamer(), clk.Now, cfg)
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestLoopStatusLimitedBy(t *testing.T) {
	r := newRig(t, rigOpts{lim: func(l *Limits) { l.MaxLinear = 0.001 }})
	r.l.SetTarget(offset(t, r.model, r3.Vector{X: 200}), [6]float64{})
	r.ticks(t, 5)
	test.That(t, r.l.Status().LimitedBy, test.ShouldContain, LimitTCPSpeed)

	r = newRig(t, rigOpts{})
	r.l.SetTarget(offset(t, r.model, r3.Vector{}), [6]float64{})
	r.ticks(t, 5)
	test.That(t, r.l.Status().LimitedBy, test.ShouldBeEmpty)
	test.That(t, r.l.Status().Limited, test.ShouldBeFalse)

	r = newRig(t, rigOpts{solver: &scriptedSolver{info: StepInfo{Damping: 0.1}}})
	r.ticks(t, 2)
	test.That(t, r.l.Status().LimitedBy, test.ShouldResemble, []Limit{LimitSingularity})
}
