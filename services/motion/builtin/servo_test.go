package builtin

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	fakearm "go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/servo"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

// noStreamArm is a fake arm whose streamed move fails at once.
type noStreamArm struct{ *fakearm.Arm }

func (noStreamArm) MoveThroughJointPositionsStreamed(
	context.Context, <-chan []arm.TrajectoryPoint, chan<- arm.Response, map[string]interface{},
) error {
	return errors.New("streaming not supported")
}

// servoHome is a non-singular UR5e pose, in radians.
var servoHome = []referenceframe.Input{0, -1.2, 1.5, -1.9, -1.57, 0}

func ptr(f float64) *float64 { return &f }

func newServoService(t *testing.T, wrap func(*fakearm.Arm) arm.Arm, conf *Config) (*builtIn, *fakearm.Arm) {
	t.Helper()
	return newServoServiceFor(t, &fakearm.Config{ArmModel: "ur5e"}, servoHome, wrap, conf)
}

// newServoServiceFor is newServoService for any fake arm config, starting from home.
func newServoServiceFor(
	t *testing.T, fc *fakearm.Config, home []referenceframe.Input, wrap func(*fakearm.Arm) arm.Arm, conf *Config,
) (*builtIn, *fakearm.Arm) {
	t.Helper()
	return newServoServiceMounted(t, fc, home, wrap, conf, spatialmath.NewZeroPose())
}

// newServoServiceMounted is newServoServiceFor with the arm mounted at mount in world.
func newServoServiceMounted(
	t *testing.T, fc *fakearm.Config, home []referenceframe.Input, wrap func(*fakearm.Arm) arm.Arm, conf *Config, mount spatialmath.Pose,
) (*builtIn, *fakearm.Arm) {
	t.Helper()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	fa, err := fakearm.NewArm(ctx, nil, resource.Config{
		Name: "arm", API: arm.API, ConvertedAttributes: fc,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	a := fa.(*fakearm.Arm)
	test.That(t, a.MoveToJointPositions(ctx, home, nil), test.ShouldBeNil)
	// The fake UR5e declares no velocity or acceleration limits; give every joint some.
	kin, err := a.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	sm := kin.(*referenceframe.SimpleModel)
	lim := map[string]referenceframe.JointLimits{}
	for _, j := range sm.ModelConfig().Joints {
		lim[j.ID] = referenceframe.JointLimits{MaxVelocity: ptr(100.0), MaxAcceleration: ptr(400.0)}
	}
	test.That(t, sm.NarrowJointLimits(lim), test.ShouldBeNil)
	var res arm.Arm = a
	if wrap != nil {
		res = wrap(a)
	}
	model, err := a.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	fs := inject.NewFrameSystemService("fs")
	fs.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: []*referenceframe.FrameSystemPart{{
			FrameConfig: referenceframe.NewLinkInFrame("world", mount, "arm", nil),
			ModelFrame:  model,
		}, {
			FrameConfig: referenceframe.NewLinkInFrame("arm", spatialmath.NewPoseFromPoint(r3.Vector{Z: 100}), "gripper", nil),
		}}}, nil
	}
	fs.CurrentInputsFunc = func(ctx context.Context) (referenceframe.FrameSystemInputs, error) {
		q, err := a.JointPositions(ctx, nil)
		return referenceframe.FrameSystemInputs{"arm": q}, err
	}
	if conf == nil {
		conf = &Config{}
	}
	svc, err := NewBuiltIn(ctx, resource.Dependencies{res.Name(): res, fs.Name(): fs},
		resource.Config{Name: "builtin", API: motion.API, ConvertedAttributes: conf}, logger)
	test.That(t, err, test.ShouldBeNil)
	return svc.(*builtIn), a
}

func do(t *testing.T, ms *builtIn, verb string, req map[string]interface{}) map[string]interface{} {
	t.Helper()
	resp, err := ms.DoCommand(context.Background(), map[string]interface{}{verb: req})
	test.That(t, err, test.ShouldBeNil)
	return resp
}

func doErr(t *testing.T, ms *builtIn, verb string, req map[string]interface{}) error {
	t.Helper()
	_, err := ms.DoCommand(context.Background(), map[string]interface{}{verb: req})
	test.That(t, err, test.ShouldNotBeNil)
	return err
}

func startServo(t *testing.T, ms *builtIn, extra map[string]interface{}) string {
	t.Helper()
	req := map[string]interface{}{"component_name": "arm", "mode": "pose", "ramp_ms": 100.0}
	for k, v := range extra {
		req[k] = v
	}
	id := do(t, ms, DoServoStart, req)["session_id"].(string)
	t.Cleanup(func() {
		_, _ = ms.DoCommand(context.Background(), map[string]interface{}{DoServoStop: map[string]interface{}{"session_id": id}})
	})
	return id
}

func poseOf(t *testing.T, a arm.Arm, dx float64) map[string]interface{} {
	t.Helper()
	p, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	o := p.Orientation().OrientationVectorDegrees()
	return map[string]interface{}{
		"x": p.Point().X + dx, "y": p.Point().Y, "z": p.Point().Z, "o_x": o.OX, "o_y": o.OY, "o_z": o.OZ, "theta": o.Theta,
	}
}

func waitState(t *testing.T, ms *builtIn, id, want string) map[string]interface{} {
	t.Helper()
	var st map[string]interface{}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st = do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})
		if st["state"] == want {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state never reached %s; last status %v", want, st)
	return nil
}

func TestServoTargetConverges(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	id := startServo(t, ms, map[string]interface{}{"timeout_ms": 8000.0})
	r := do(t, ms, DoServoTarget, map[string]interface{}{
		"session_id": id, "seq": 1.0, "target": map[string]interface{}{"pose": poseOf(t, a, 20)},
	})
	test.That(t, r["applied"], test.ShouldBeTrue)
	st := waitState(t, ms, id, "converged")
	test.That(t, st["mode"], test.ShouldEqual, "pose")
	test.That(t, st["last_seq"], test.ShouldEqual, int64(1))
	test.That(t, len(st["q_meas_deg"].([]float64)), test.ShouldEqual, 6)
	test.That(t, st["limited_by"], test.ShouldHaveSameTypeAs, []string{})
	// stale seq is dropped; a seq of 0 or a fraction is an error
	r = do(t, ms, DoServoTarget, map[string]interface{}{
		"session_id": id, "seq": 1.0, "target": map[string]interface{}{"pose": poseOf(t, a, 30)},
	})
	test.That(t, r["applied"], test.ShouldBeFalse)
	for _, bad := range []float64{0, 1.5, -2} {
		doErr(t, ms, DoServoTarget, map[string]interface{}{
			"session_id": id, "seq": bad, "target": map[string]interface{}{"pose": poseOf(t, a, 30)},
		})
	}
	// an unknown reference frame and a wrong-mode call fail
	doErr(t, ms, DoServoTarget, map[string]interface{}{
		"session_id": id, "seq": 2.0, "target": map[string]interface{}{"reference_frame": "nope", "pose": poseOf(t, a, 5)},
	})
	doErr(t, ms, DoServoVelocity, map[string]interface{}{"session_id": id})
}

func TestServoVelocity(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	id := startServo(t, ms, map[string]interface{}{"mode": "velocity"})
	before, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 15; i++ {
		r := do(t, ms, DoServoVelocity, map[string]interface{}{
			"session_id": id, "linear_mm_per_sec": map[string]interface{}{"x": 50.0},
		})
		test.That(t, r["applied"], test.ShouldBeTrue)
		time.Sleep(20 * time.Millisecond)
	}
	after, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, after.Point().X-before.Point().X, test.ShouldBeGreaterThan, 1.0)
	doErr(t, ms, DoServoTarget, map[string]interface{}{"session_id": id, "target": map[string]interface{}{}})
}

func TestServoStopBySessionAndComponent(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	id := startServo(t, ms, nil)
	r := do(t, ms, DoServoStop, map[string]interface{}{"session_id": id})
	test.That(t, r["state"], test.ShouldEqual, "stopped")
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
	// an ended session still reports its terminal state, and refuses commands with it
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "stopped")
	err := doErr(t, ms, DoServoTarget, map[string]interface{}{"session_id": id, "target": map[string]interface{}{"pose": poseOf(t, a, 1)}})
	test.That(t, err.Error(), test.ShouldContainSubstring, "stopped")

	startServo(t, ms, nil)
	r = do(t, ms, DoServoStop, map[string]interface{}{"component_name": "arm"})
	test.That(t, r["state"], test.ShouldEqual, "stopped")
	doErr(t, ms, DoServoStop, map[string]interface{}{})
}

func TestServoStartHeldArm(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	startServo(t, ms, nil)
	err := doErr(t, ms, DoServoStart, map[string]interface{}{"component_name": "arm", "mode": "pose"})
	test.That(t, err.Error(), test.ShouldContainSubstring, `arm "arm" is held by a servo session`)
	// a TempStream would be refused the same way
	test.That(t, ms.registerStream("arm", &stream{}), test.ShouldNotBeNil)
}

func TestServoStartValidation(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	base := func(k string, v interface{}) map[string]interface{} {
		return map[string]interface{}{"component_name": "arm", "mode": "pose", k: v}
	}
	for _, req := range []map[string]interface{}{
		{"mode": "pose"},
		{"component_name": "nope", "mode": "pose"},
		{"component_name": "arm", "mode": "wiggle"},
		base("timeout_ms", 20000.0),
		base("ramp_ms", 2000.0),
		base("ramp_ms", -0.5),
		base("ramp_ms", math.NaN()),
		base("component_name", 5.0),
		base("extra", map[string]interface{}{"solver": 3.0}),
		base("limits", map[string]interface{}{"max_vel_degs_per_sec": -1.0}),
		base("limits", map[string]interface{}{"max_acc_degs_per_sec2_joints": []interface{}{math.Inf(1), 1.0, 1.0, 1.0, 1.0, 1.0}}),
		base("limits", map[string]interface{}{"max_tcp_speed": math.NaN()}),
		base("max_target_step_mm", 0.0),
		base("max_target_step_deg", -1.0),
		base("extra", map[string]interface{}{"solver": "magic"}),
		base("extra", map[string]interface{}{"resync_deg": 0.0}),
		base("extra", map[string]interface{}{"resync_deg": -1.0}),
		base("extra", map[string]interface{}{"resync_deg": math.NaN()}),
		base("extra", map[string]interface{}{"resync_deg": "x"}),
	} {
		doErr(t, ms, DoServoStart, req)
	}
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
}

func TestServoLimitsLooserThanModel(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	err := doErr(t, ms, DoServoStart, map[string]interface{}{
		"component_name": "arm", "mode": "pose",
		"limits": map[string]interface{}{"max_vel_degs_per_sec": 100000.0},
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "above the model's")
	// tighter limits are reported back
	id := startServo(t, ms, map[string]interface{}{"limits": map[string]interface{}{"max_vel_degs_per_sec": 10.0, "max_tcp_speed": 0.05}})
	st := do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})
	lim := st["limits"].(map[string]interface{})
	for _, v := range lim["max_vel_degs_per_sec_joints"].([]float64) {
		test.That(t, v, test.ShouldBeLessThanOrEqualTo, 10.0+1e-9)
	}
	test.That(t, lim["max_tcp_speed"], test.ShouldEqual, 0.05)
}

func TestServoInputRangeOverrideClamped(t *testing.T) {
	ctx := context.Background()
	// Baseline: the arm's own joint 0 range.
	_, a := newServoService(t, nil, nil)
	model, err := a.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	own := model.DoF()[0]
	wide := referenceframe.Limit{Min: own.Min - 1, Max: own.Max + 1}
	ms, _ := newServoService(t, nil, &Config{InputRangeOverride: map[string]map[string]referenceframe.Limit{"arm": {"0": wide}}})
	defer ms.Close(ctx)
	id := startServo(t, ms, nil)
	lim := do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["limits"].(map[string]interface{})
	test.That(t, lim["min_deg"].([]float64)[0], test.ShouldAlmostEqual, own.Min*180/3.141592653589793, 1e-6)
	test.That(t, lim["max_deg"].([]float64)[0], test.ShouldAlmostEqual, own.Max*180/3.141592653589793, 1e-6)
}

func TestServoStartArmCannotStream(t *testing.T) {
	ms, _ := newServoService(t, func(a *fakearm.Arm) arm.Arm { return noStreamArm{a} }, nil)
	err := doErr(t, ms, DoServoStart, map[string]interface{}{"component_name": "arm", "mode": "pose"})
	test.That(t, err.Error(), test.ShouldContainSubstring, "MoveThroughJointPositionsStreamed")
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
}

func TestServoDeadMan(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	id := startServo(t, ms, map[string]interface{}{"timeout_ms": 100.0})
	waitState(t, ms, id, "stopped_timeout")
}

// countingArm counts MoveThroughJointPositionsStreamed calls.
type countingArm struct {
	*fakearm.Arm
	calls *atomic.Int32
}

func (c countingArm) MoveThroughJointPositionsStreamed(
	ctx context.Context, b <-chan []arm.TrajectoryPoint, r chan<- arm.Response, extra map[string]interface{},
) error {
	c.calls.Add(1)
	return c.Arm.MoveThroughJointPositionsStreamed(ctx, b, r, extra)
}

func TestServoConcurrentStartsOpenOneStream(t *testing.T) {
	var calls atomic.Int32
	ms, _ := newServoService(t, func(a *fakearm.Arm) arm.Arm { return countingArm{a, &calls} }, nil)
	const n = 4
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ms.DoCommand(context.Background(), map[string]interface{}{
				DoServoStart: map[string]interface{}{"component_name": "arm", "mode": "pose"},
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		} else {
			test.That(t, err.Error(), test.ShouldContainSubstring, "held by a servo session")
		}
	}
	test.That(t, ok, test.ShouldEqual, 1)
	test.That(t, calls.Load(), test.ShouldEqual, 1)
	do(t, ms, DoServoStop, map[string]interface{}{"component_name": "arm"})
}

func TestRegistryReleaseUnpublishedLeavesNoLast(t *testing.T) {
	var r registry
	e := &servoEntry{arm: "arm"}
	test.That(t, r.claim("arm", holder{kind: holderServo, servo: e}), test.ShouldBeNil)
	r.release("arm", holderServo)
	test.That(t, r.ended(), test.ShouldBeEmpty)
}

func teleopReq(t *testing.T, ms *builtIn, name string) motion.MoveReq {
	t.Helper()
	a := ms.components["arm"].(arm.Arm)
	p, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	return motion.MoveReq{ComponentName: name, Destination: referenceframe.NewPoseInFrame("world", p)}
}

func TestTeleopServoExclusion(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	test.That(t, ms.addTeleopComponent(context.Background(), teleopReq(t, ms, "arm")), test.ShouldBeNil)
	h, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, h.kind, test.ShouldEqual, holderTeleop)
	err := doErr(t, ms, DoServoStart, map[string]interface{}{"component_name": "arm", "mode": "pose"})
	test.That(t, err.Error(), test.ShouldContainSubstring, "held by teleop")
	// re-claiming its own arm is a no-op (pipeline exists: uses the cached frame system)
	test.That(t, ms.addTeleopComponent(context.Background(), teleopReq(t, ms, "arm")), test.ShouldBeNil)
	do(t, ms, DoTeleopStop, map[string]interface{}{"component_name": "arm"}) // last component: pipeline stops
	_, ok = ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
	startServo(t, ms, nil)

	// teleop on a servo-held arm errors, leaving no pipeline behind
	err = ms.addTeleopComponent(context.Background(), teleopReq(t, ms, "arm"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "held by a servo session")
	test.That(t, ms.teleopPipeline, test.ShouldBeNil)
	do(t, ms, DoServoStop, map[string]interface{}{"component_name": "arm"})

	// Reconfigure stops teleop and frees the arm
	test.That(t, ms.addTeleopComponent(context.Background(), teleopReq(t, ms, "arm")), test.ShouldBeNil)
	test.That(t, ms.BuiltInReconfigure(context.Background(), resource.Dependencies{}, resource.Config{
		Name: "builtin", API: motion.API, ConvertedAttributes: &Config{},
	}), test.ShouldBeNil)
	_, ok = ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
}

// moveRecorder records, at the first move on the arm, whether the servo session had already ended.
type moveRecorder struct {
	*fakearm.Arm
	once    sync.Once
	ms      *builtIn
	session *servoEntry
	state   atomic.Value
	held    atomic.Bool
}

func (m *moveRecorder) record() {
	m.once.Do(func() {
		_, ok := m.ms.reg.get("arm")
		m.held.Store(ok)
		m.state.Store(string(m.session.session.Status().State))
	})
}

func (m *moveRecorder) MoveToJointPositions(ctx context.Context, j []referenceframe.Input, e map[string]interface{}) error {
	m.record()
	return m.Arm.MoveToJointPositions(ctx, j, e)
}

func (m *moveRecorder) GoToInputs(ctx context.Context, in ...[]referenceframe.Input) error {
	m.record()
	return m.Arm.GoToInputs(ctx, in...)
}

func TestMoveStopsServoSession(t *testing.T) {
	var rec *moveRecorder
	ms, a := newServoService(t, func(fa *fakearm.Arm) arm.Arm { rec = &moveRecorder{Arm: fa}; return rec }, nil)
	rec.ms = ms
	id := startServo(t, ms, nil)
	rec.session = func() *servoEntry { e, _ := ms.reg.getServo("arm"); return &e }()
	p, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	dest := spatialmath.NewPose(p.Point().Add(r3.Vector{Z: 20}), p.Orientation())
	ok, err := ms.Move(context.Background(), motion.MoveReq{
		ComponentName: "gripper", Destination: referenceframe.NewPoseInFrame("world", dest),
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rec.state.Load(), test.ShouldEqual, "stopped")
	test.That(t, rec.held.Load(), test.ShouldBeFalse)
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "stopped")
}

func TestMoveRefusedByStreamAndPendingServo(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	req := motion.MoveReq{ComponentName: "gripper", Destination: referenceframe.NewPoseInFrame("world", spatialmath.NewZeroPose())}
	test.That(t, ms.registerStream("arm", &stream{}), test.ShouldBeNil)
	_, err := ms.Move(context.Background(), req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `arm "arm" is held by an arm-streaming session`)
	ms.unregisterStream("arm")

	test.That(t, ms.reg.claim("arm", holder{kind: holderServo, servo: &servoEntry{arm: "arm"}}), test.ShouldBeNil)
	_, err = ms.Move(context.Background(), req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `arm "arm" is starting a servo session`)
}

func TestStopPlanServo(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	err := ms.StopPlan(context.Background(), motion.StopPlanReq{ComponentName: "arm"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "StopPlan not supported by builtin")
	id := startServo(t, ms, nil)
	test.That(t, ms.StopPlan(context.Background(), motion.StopPlanReq{ComponentName: "arm"}), test.ShouldBeNil)
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "stopped")
}

func TestClaimTeleopArmsWalksChain(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	fs, err := ms.getFrameSystem(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	arms, err := ms.claimTeleopArms(fs, "gripper")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arms, test.ShouldResemble, []string{"arm"})
	h, _ := ms.reg.get("arm")
	test.That(t, h.kind, test.ShouldEqual, holderTeleop)
}

// stoppableArm wraps the fake arm; its Stop ends the open stream, as a real arm's would.
type stoppableArm struct {
	arm.Arm
	mu   sync.Mutex
	stop chan struct{}
}

func (a *stoppableArm) MoveThroughJointPositionsStreamed(ctx context.Context, b <-chan []arm.TrajectoryPoint,
	r chan<- arm.Response, extra map[string]interface{},
) error {
	a.mu.Lock()
	a.stop = make(chan struct{})
	stop := a.stop
	a.mu.Unlock()
	for {
		select {
		case <-stop:
			return errors.New("arm stopped")
		case batch, ok := <-b:
			if !ok {
				return nil
			}
			// Forward to the fake so its joints follow the commands; otherwise the loop resyncs every tick.
			if len(batch) > 0 {
				if err := a.Arm.MoveToJointPositions(ctx, batch[len(batch)-1].Positions, nil); err != nil {
					return err
				}
			}
		}
	}
}

func (a *stoppableArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	return nil
}

func newStoppable(t *testing.T) (*builtIn, *stoppableArm) {
	t.Helper()
	var sa *stoppableArm
	ms, _ := newServoService(t, func(f *fakearm.Arm) arm.Arm { sa = &stoppableArm{Arm: f}; return sa }, nil)
	return ms, sa
}

func TestServoIsMoving(t *testing.T) {
	ms, _ := newStoppable(t)
	moving, err := ms.IsMoving(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, moving, test.ShouldBeFalse)
	id := startServo(t, ms, nil)
	moving, _ = ms.IsMoving(context.Background())
	test.That(t, moving, test.ShouldBeTrue)
	do(t, ms, DoServoStop, map[string]interface{}{"session_id": id})
	moving, _ = ms.IsMoving(context.Background())
	test.That(t, moving, test.ShouldBeFalse)
}

func TestServoStopAllMotionFirst(t *testing.T) {
	ms, sa := newStoppable(t)
	id := startServo(t, ms, nil)
	test.That(t, ms.Stop(context.Background(), nil), test.ShouldBeNil)
	test.That(t, sa.Stop(context.Background(), nil), test.ShouldBeNil)
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "stopped")
}

func TestServoStopAllArmFirst(t *testing.T) {
	ms, sa := newStoppable(t)
	id := startServo(t, ms, nil)
	test.That(t, sa.Stop(context.Background(), nil), test.ShouldBeNil)
	time.Sleep(50 * time.Millisecond)
	test.That(t, ms.Stop(context.Background(), nil), test.ShouldBeNil)
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "stopped")
}

func TestServoArmStopAloneFaults(t *testing.T) {
	ms, sa := newStoppable(t)
	id := startServo(t, ms, nil)
	test.That(t, sa.Stop(context.Background(), nil), test.ShouldBeNil)
	waitState(t, ms, id, "faulted")
}

func reconfigure(t *testing.T, ms *builtIn, res resource.Resource) {
	t.Helper()
	deps := resource.Dependencies{res.Name(): res, ms.fsService.Name(): ms.fsService}
	test.That(t, ms.BuiltInReconfigure(context.Background(), deps,
		resource.Config{Name: "builtin", API: motion.API, ConvertedAttributes: &Config{}}), test.ShouldBeNil)
}

func TestServoReconfigureKeepsOrStops(t *testing.T) {
	ms, sa := newStoppable(t)
	id := startServo(t, ms, nil)
	reconfigure(t, ms, sa) // same resource
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "running")

	reconfigure(t, ms, &stoppableArm{Arm: sa.Arm}) // a replaced arm
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldEqual, "stopped")
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
}

func TestServoReconfigureRemovedArmForgets(t *testing.T) {
	ms, _ := newStoppable(t)
	startServo(t, ms, nil)
	test.That(t, ms.BuiltInReconfigure(context.Background(), resource.Dependencies{ms.fsService.Name(): ms.fsService},
		resource.Config{Name: "builtin", API: motion.API, ConvertedAttributes: &Config{}}), test.ShouldBeNil)
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, ms.reg.ended(), test.ShouldBeEmpty)
}

func TestServoClose(t *testing.T) {
	ms, _ := newStoppable(t)
	startServo(t, ms, nil)
	e, _ := ms.reg.getServo("arm")
	test.That(t, ms.Close(context.Background()), test.ShouldBeNil)
	test.That(t, e.session.Status().State, test.ShouldEqual, servo.StateStopped)
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, ms.reg.ended(), test.ShouldBeEmpty)
}

// gateArm blocks its streamed move until released, holding a start between claim and publish.
type gateArm struct {
	*fakearm.Arm
	entered chan struct{}
	release chan struct{}
}

func (g gateArm) MoveThroughJointPositionsStreamed(
	ctx context.Context, b <-chan []arm.TrajectoryPoint, r chan<- arm.Response, extra map[string]interface{},
) error {
	close(g.entered)
	<-g.release
	return g.Arm.MoveThroughJointPositionsStreamed(ctx, b, r, extra)
}

func pendingStartRefused(t *testing.T, act func(ms *builtIn, a *fakearm.Arm), want string) {
	g := gateArm{entered: make(chan struct{}), release: make(chan struct{})}
	ms, fa := newServoService(t, func(f *fakearm.Arm) arm.Arm { g.Arm = f; return g }, nil)
	errc := make(chan error, 1)
	go func() {
		_, err := ms.DoCommand(context.Background(), map[string]interface{}{
			DoServoStart: map[string]interface{}{"component_name": "arm", "mode": "pose"},
		})
		errc <- err
	}()
	<-g.entered
	act(ms, fa)
	close(g.release)
	err := <-errc
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, want)
	_, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, ms.reg.all(holderServo), test.ShouldBeEmpty)
}

func TestServoCloseRefusesPendingStart(t *testing.T) {
	pendingStartRefused(t, func(ms *builtIn, _ *fakearm.Arm) {
		test.That(t, ms.Close(context.Background()), test.ShouldBeNil)
	}, "closed")
}

func TestServoReconfigureRefusesPendingStart(t *testing.T) {
	pendingStartRefused(t, func(ms *builtIn, fa *fakearm.Arm) {
		reconfigure(t, ms, &stoppableArm{Arm: fa})
	}, "reconfigured")
}

func TestServoStopRefusesPendingStart(t *testing.T) {
	pendingStartRefused(t, func(ms *builtIn, _ *fakearm.Arm) {
		test.That(t, ms.Stop(context.Background(), nil), test.ShouldBeNil)
	}, "stopped during servo_start")
}

func TestStopPlanPendingStart(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	e := &servoEntry{arm: "arm"}
	test.That(t, ms.reg.claim("arm", holder{kind: holderServo, servo: e}), test.ShouldBeNil)
	err := ms.StopPlan(context.Background(), motion.StopPlanReq{ComponentName: "arm"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "starting a servo session")
}

func stopTeleop(t *testing.T, ms *builtIn, name string) {
	t.Helper()
	_, err := ms.DoCommand(context.Background(), map[string]interface{}{DoTeleopStop: true, "component_name": name})
	test.That(t, err, test.ShouldBeNil)
}

func TestTeleopTwoComponentsShareArm(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	ctx := context.Background()
	ms.components["gripper"] = ms.components["arm"] // teleop only checks that the component exists
	test.That(t, ms.addTeleopComponent(ctx, teleopReq(t, ms, "arm")), test.ShouldBeNil)
	test.That(t, ms.addTeleopComponent(ctx, teleopReq(t, ms, "gripper")), test.ShouldBeNil)
	stopTeleop(t, ms, "gripper")
	h, ok := ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, h.kind, test.ShouldEqual, holderTeleop)
	stopTeleop(t, ms, "arm")
	_, ok = ms.reg.get("arm")
	test.That(t, ok, test.ShouldBeFalse)
}

func TestProbePointLengths(t *testing.T) {
	p := probePoint(servoHome)
	test.That(t, len(p.Velocities), test.ShouldEqual, len(servoHome))
	test.That(t, len(p.Accelerations), test.ShouldEqual, len(servoHome))
}

func TestServoResyncDegAccepted(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	startServo(t, ms, map[string]interface{}{"extra": map[string]interface{}{"resync_deg": 5.0}})
}

// A 5-DOF arm runs the pointing task: a roll-only change beyond max_target_step_deg is not a step.
func TestServoFiveDOFRollOnlyTarget(t *testing.T) {
	ms, a := newServoServiceFor(t, &fakearm.Config{ModelFilePath: "servo/testdata/so101.urdf"},
		[]referenceframe.Input{0, 0.3, 0.3, 0.5, 0}, nil, nil)
	id := startServo(t, ms, map[string]interface{}{"timeout_ms": 8000.0})
	pose := poseOf(t, a, 0)
	pose["theta"] = pose["theta"].(float64) + 60
	r := do(t, ms, DoServoTarget, map[string]interface{}{
		"session_id": id, "seq": 1.0, "target": map[string]interface{}{"pose": pose},
	})
	test.That(t, r["applied"], test.ShouldBeTrue)
}

func TestRegistryClaimAfterClose(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	test.That(t, ms.Close(context.Background()), test.ShouldBeNil)
	err := doErr(t, ms, DoServoStart, map[string]interface{}{"component_name": "arm", "mode": "pose"})
	test.That(t, err.Error(), test.ShouldContainSubstring, "closed")
}

func TestServoStreamedSessionRefusesDoCommandWrites(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	started, err := ms.startServo(context.Background(), servoParams{
		arm: "arm", mode: servo.ModePose, maxAngDegs: 45, timeoutMS: 500, rampMS: 100, stepMM: 100, stepDeg: 30,
		solver: "dls", streamed: true,
	})
	test.That(t, err, test.ShouldBeNil)
	id := started.entry.id
	t.Cleanup(func() {
		_, _ = ms.DoCommand(context.Background(), map[string]interface{}{DoServoStop: map[string]interface{}{"session_id": id}})
	})
	for verb, req := range map[string]map[string]interface{}{
		DoServoTarget:   {"seq": 1.0, "target": map[string]interface{}{"pose": poseOf(t, a, 5)}},
		DoServoVelocity: {"seq": 1.0},
	} {
		req["component_name"] = "arm"
		test.That(t, doErr(t, ms, verb, req).Error(), test.ShouldContainSubstring, "owned by a StreamServo call")
	}
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"session_id": id})["state"], test.ShouldNotBeEmpty)
}
