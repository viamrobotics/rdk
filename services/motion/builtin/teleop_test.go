package builtin

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const (
	teleopTestArm = "pieceArm"
	// teleopTestComponent is the frame teleop moves. It is carried by teleopTestArm, so the
	// teleop component's name differs from the name of the arm that is actuated.
	teleopTestComponent = "pieceGripper"
	teleopTestTimeout   = 30 * time.Second

	teleopTestMaxVelDegs = 60.0
	teleopTestMaxAccDegs = 300.0
)

// fakeTeleopArmStreamRunner stands in for streaming.Run. It records the seed and every target
// it is handed, and
// returns when targets is closed, when ctx is cancelled, or with failErr right after the first
// target if failErr is set.
type fakeTeleopArmStreamRunner struct {
	mu            sync.Mutex
	runs          int
	returns       int
	closedCleanly int
	seeds         [][]referenceframe.Input
	targets       [][]referenceframe.Input
	opts          []streaming.StreamOptions
	failErr       error
}

func (f *fakeTeleopArmStreamRunner) run(
	ctx context.Context,
	_ logging.Logger,
	_ arm.Arm,
	opts streaming.StreamOptions,
	targets <-chan []referenceframe.Input,
	seed []referenceframe.Input,
	diag *diagnostics.SingleSessionDiagnostics,
) error {
	f.mu.Lock()
	f.runs++
	f.seeds = append(f.seeds, seed)
	f.opts = append(f.opts, opts)
	failErr := f.failErr
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.returns++
		f.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case jp, ok := <-targets:
			if !ok {
				f.mu.Lock()
				f.closedCleanly++
				f.mu.Unlock()
				return nil
			}
			f.mu.Lock()
			f.targets = append(f.targets, jp)
			f.mu.Unlock()
			diag.RecordReceivedJointPositionTargetEvent()
			if failErr != nil {
				return failErr
			}
		}
	}
}

// fakeRunnerState is a consistent copy of a fakeTeleopArmStreamRunner's counters and records.
type fakeRunnerState struct {
	runs, returns, closedCleanly int
	seeds, targets               [][]referenceframe.Input
}

func (f *fakeTeleopArmStreamRunner) snapshot() fakeRunnerState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeRunnerState{
		runs:          f.runs,
		returns:       f.returns,
		closedCleanly: f.closedCleanly,
		seeds:         append([][]referenceframe.Input(nil), f.seeds...),
		targets:       append([][]referenceframe.Input(nil), f.targets...),
	}
}

func setupTeleopTest(t *testing.T) (*builtIn, arm.Arm, func()) {
	t.Helper()
	svc, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	ms, ok := svc.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)
	ms.mu.RLock()
	r := ms.components[teleopTestArm]
	ms.mu.RUnlock()
	a, ok := r.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	// The fake arm publishes no joint velocity or acceleration limits, so give teleop some to
	// stream under. Tests of the no-limits case clear these.
	ms.mu.Lock()
	ms.conf.TeleopStream.MaxVelDegsPerSec = teleopTestMaxVelDegs
	ms.conf.TeleopStream.MaxAccDegsPerSec2 = teleopTestMaxAccDegs
	ms.mu.Unlock()
	return ms, a, teardown
}

func teleopStart(t *testing.T, ms *builtIn) *referenceframe.PoseInFrame {
	t.Helper()
	ctx := context.Background()
	pose, err := ms.GetPose(ctx, teleopTestComponent, referenceframe.World, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	req := motion.MoveReq{ComponentName: teleopTestComponent, Destination: pose}
	proto, err := req.ToProto(ms.Name().Name)
	test.That(t, err, test.ShouldBeNil)
	bytes, err := protojson.Marshal(proto)
	test.That(t, err, test.ShouldBeNil)
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoTeleopStart: string(bytes)})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[DoTeleopStart], test.ShouldBeTrue)
	return pose
}

func teleopMove(t *testing.T, ms *builtIn, pose *referenceframe.PoseInFrame, seq int) {
	t.Helper()
	bytes, err := protojson.Marshal(referenceframe.PoseInFrameToProtobuf(pose))
	test.That(t, err, test.ShouldBeNil)
	resp, err := ms.DoCommand(context.Background(), map[string]interface{}{
		DoTeleopMove:     string(bytes),
		"component_name": teleopTestComponent,
		"seq":            float64(seq),
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[DoTeleopMove], test.ShouldBeTrue)
}

func teleopStop(t *testing.T, ms *builtIn) {
	t.Helper()
	resp, err := ms.DoCommand(context.Background(), map[string]interface{}{DoTeleopStop: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[DoTeleopStop], test.ShouldBeTrue)
}

func teleopStatus(t *testing.T, ms *builtIn) map[string]any {
	t.Helper()
	resp, err := ms.DoCommand(context.Background(), map[string]interface{}{DoTeleopStatus: true})
	test.That(t, err, test.ShouldBeNil)
	st, ok := resp[DoTeleopStatus].(map[string]any)
	test.That(t, ok, test.ShouldBeTrue)
	return st
}

func armStreamStatus(t *testing.T, ms *builtIn) map[string]any {
	t.Helper()
	streams, ok := teleopStatus(t, ms)["streams"].(map[string]any)
	if !ok {
		return nil
	}
	info, _ := streams[teleopTestArm].(map[string]any)
	return info
}

// offsetPose returns pose shifted by dz millimeters along world Z.
func offsetPose(pose *referenceframe.PoseInFrame, dz float64) *referenceframe.PoseInFrame {
	p := pose.Pose()
	pt := p.Point()
	shifted := spatialmath.NewPose(r3.Vector{X: pt.X, Y: pt.Y, Z: pt.Z + dz}, p.Orientation())
	return referenceframe.NewPoseInFrame(pose.Parent(), shifted)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(teleopTestTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestTeleopStreamedExecutor(t *testing.T) {
	ms, a, teardown := setupTeleopTest(t)
	defer teardown()
	ctx := context.Background()

	fake := &fakeTeleopArmStreamRunner{}
	ms.teleopRunArmStream = fake.run
	diagDir := t.TempDir()
	ms.mu.Lock()
	ms.conf.StreamDiagnosticsFilePath = diagDir
	ms.mu.Unlock()

	initial, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	pose := teleopStart(t, ms)
	st := teleopStatus(t, ms)
	test.That(t, st["running"], test.ShouldBeTrue)

	teleopMove(t, ms, offsetPose(pose, 5), 1)
	waitFor(t, "first target to reach the arm stream", func() bool {
		return len(fake.snapshot().targets) > 0
	})

	state := fake.snapshot()
	test.That(t, state.runs, test.ShouldEqual, 1)
	// The stream is seeded from where the arm actually is, and every target has the arm's DoF.
	test.That(t, state.seeds[0], test.ShouldResemble, initial)
	// The stream is given one velocity and one acceleration limit per joint.
	fake.mu.Lock()
	streamOpts := fake.opts[0]
	fake.mu.Unlock()
	test.That(t, len(streamOpts.MoveOptions.MaxVelRadsJoints), test.ShouldEqual, len(initial))
	test.That(t, streamOpts.MoveOptions.MaxVelRadsJoints[0], test.ShouldAlmostEqual, utils.DegToRad(teleopTestMaxVelDegs), 1e-9)
	test.That(t, streamOpts.MoveOptions.MaxAccRadsJoints[0], test.ShouldAlmostEqual, utils.DegToRad(teleopTestMaxAccDegs), 1e-9)
	for _, target := range state.targets {
		test.That(t, len(target), test.ShouldEqual, len(initial))
	}

	info := armStreamStatus(t, ms)
	test.That(t, info, test.ShouldNotBeNil)
	test.That(t, info["open"], test.ShouldBeTrue)
	test.That(t, info["failed"], test.ShouldBeFalse)

	// Teleop never calls the arm's unary move, so the fake arm has not been teleported.
	after, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, after, test.ShouldResemble, initial)

	// Give the trace's sampler time to read the arm at least once.
	time.Sleep(5 * teleopTraceSampleInterval)

	// Stopping closes the stream gracefully: the target channel is closed, not the context.
	teleopStop(t, ms)
	waitFor(t, "arm stream to drain and return", func() bool {
		return fake.snapshot().returns == 1
	})
	test.That(t, fake.snapshot().closedCleanly, test.ShouldEqual, 1)
	test.That(t, teleopStatus(t, ms)["running"], test.ShouldBeFalse)

	// The teleop trace holds the goals sent and the targets commanded.
	traces, err := filepath.Glob(filepath.Join(diagDir, "tag=teleop-trace", "teleop-trace-*.jsonl"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(traces), test.ShouldEqual, 1)
	traceBytes, err := os.ReadFile(traces[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(traceBytes), test.ShouldContainSubstring, `"k":"goal","n":"`+teleopTestComponent+`"`)
	test.That(t, string(traceBytes), test.ShouldContainSubstring, `"k":"cmd","n":"`+teleopTestArm+`"`)
	test.That(t, string(traceBytes), test.ShouldContainSubstring, `"k":"act","n":"`+teleopTestArm+`"`)

	// The stream's diagnostics file is written when it ends.
	pattern := filepath.Join(diagDir, "tag=teleop-arm-stream-diagnostics", "tag="+teleopTestArm, "*.json")
	waitFor(t, "diagnostics file to be written", func() bool {
		files, err := filepath.Glob(pattern)
		test.That(t, err, test.ShouldBeNil)
		return len(files) == 1
	})
}

func TestTeleopArmWithoutStreamingFails(t *testing.T) {
	ms, a, teardown := setupTeleopTest(t)
	defer teardown()
	ctx := context.Background()

	fake := &fakeTeleopArmStreamRunner{failErr: status.Error(codes.Unimplemented, "no streaming here")}
	ms.teleopRunArmStream = fake.run

	initial, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	pose := teleopStart(t, ms)

	// Keep nudging the goal until the executor has seen the stream die and given up on the arm.
	seq := 0
	waitFor(t, "executor to give up on the arm", func() bool {
		seq++
		teleopMove(t, ms, offsetPose(pose, float64(seq)), seq)
		info := armStreamStatus(t, ms)
		return info != nil && info["failed"] == true
	})
	info := armStreamStatus(t, ms)
	test.That(t, info["failure_reason"], test.ShouldContainSubstring, "does not implement")
	test.That(t, info["open"], test.ShouldBeFalse)

	// Later trajectories report the failure and neither reopen the stream nor move the arm.
	execsAtFailure := teleopStatus(t, ms)["exec_count"].(int64)
	waitFor(t, "a trajectory to be refused after the failure", func() bool {
		seq++
		teleopMove(t, ms, offsetPose(pose, float64(seq)), seq)
		st := teleopStatus(t, ms)
		errText, _ := st["error"].(string)
		return st["exec_count"].(int64) > execsAtFailure && errText != ""
	})
	test.That(t, teleopStatus(t, ms)["error"], test.ShouldContainSubstring, "does not implement")
	test.That(t, fake.snapshot().runs, test.ShouldEqual, 1)
	after, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, after, test.ShouldResemble, initial)

	teleopStop(t, ms)
}

func TestTeleopStreamJointLimits(t *testing.T) {
	limit := func(velDegs, accDegs float64) referenceframe.Limit {
		vel, acc := utils.DegToRad(velDegs), utils.DegToRad(accDegs)
		return referenceframe.Limit{Min: -3, Max: 3, MaxVelocity: &vel, MaxAcceleration: &acc}
	}
	published := []referenceframe.Limit{limit(180, 1000), limit(90, 400)}
	unpublished := []referenceframe.Limit{{Min: -3, Max: 3}, {Min: -3, Max: 3}}
	degs := func(rads []float64) []float64 {
		out := make([]float64, len(rads))
		for i, r := range rads {
			out[i] = utils.RadToDeg(r)
		}
		return out
	}
	shouldBe := func(got []float64, want ...float64) {
		t.Helper()
		test.That(t, len(got), test.ShouldEqual, len(want))
		for i := range want {
			test.That(t, got[i], test.ShouldAlmostEqual, want[i], 1e-6)
		}
	}

	// With nothing configured, the arm's own limits are used, less the margin.
	vel, acc, err := TeleopStreamConfig{}.jointLimits(published)
	test.That(t, err, test.ShouldBeNil)
	shouldBe(degs(vel), 180*teleopStreamLimitMargin, 90*teleopStreamLimitMargin)
	shouldBe(degs(acc), 1000*teleopStreamLimitMargin, 400*teleopStreamLimitMargin)

	// A configured value lowers a joint's limit but never raises it past the arm's.
	vel, acc, err = TeleopStreamConfig{MaxVelDegsPerSec: 100, MaxAccDegsPerSec2: 5000}.jointLimits(published)
	test.That(t, err, test.ShouldBeNil)
	shouldBe(degs(vel), 100, 90*teleopStreamLimitMargin)
	shouldBe(degs(acc), 1000*teleopStreamLimitMargin, 400*teleopStreamLimitMargin)

	// An arm that publishes no limits gets the configured ones, and is refused without them.
	vel, acc, err = TeleopStreamConfig{MaxVelDegsPerSec: 50, MaxAccDegsPerSec2: 200}.jointLimits(unpublished)
	test.That(t, err, test.ShouldBeNil)
	shouldBe(degs(vel), 50, 50)
	shouldBe(degs(acc), 200, 200)
	_, _, err = TeleopStreamConfig{MaxVelDegsPerSec: 50}.jointLimits(unpublished)
	test.That(t, err, test.ShouldBeError, errTeleopNoStreamLimits)
	_, _, err = TeleopStreamConfig{}.jointLimits(unpublished)
	test.That(t, err, test.ShouldBeError, errTeleopNoStreamLimits)
}

func TestTeleopArmWithoutLimitsFails(t *testing.T) {
	ms, a, teardown := setupTeleopTest(t)
	defer teardown()
	ctx := context.Background()

	fake := &fakeTeleopArmStreamRunner{}
	ms.teleopRunArmStream = fake.run
	ms.mu.Lock()
	ms.conf.TeleopStream = TeleopStreamConfig{}
	ms.mu.Unlock()

	initial, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	pose := teleopStart(t, ms)
	seq := 0
	waitFor(t, "executor to refuse the arm", func() bool {
		seq++
		teleopMove(t, ms, offsetPose(pose, float64(seq)), seq)
		info := armStreamStatus(t, ms)
		return info != nil && info["failed"] == true
	})
	info := armStreamStatus(t, ms)
	test.That(t, info["failure_reason"], test.ShouldContainSubstring, "publishes no joint velocity")
	test.That(t, fake.snapshot().runs, test.ShouldEqual, 0)
	after, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, after, test.ShouldResemble, initial)

	teleopStop(t, ms)
}

func TestTeleopPlanJump(t *testing.T) {
	rads := func(degs ...float64) []referenceframe.Input {
		out := make([]referenceframe.Input, len(degs))
		for i, d := range degs {
			out[i] = utils.DegToRad(d)
		}
		return out
	}
	arms := map[string]bool{"arm": true}
	head := referenceframe.FrameSystemInputs{"arm": rads(42.9, -34.0, 8.0, -2.5, 20.9, 42.9), "gripper": rads(0)}

	// An ordinary step, a few degrees after 10ms, is accepted.
	next := referenceframe.FrameSystemInputs{"arm": rads(43.3, -34.4, 8.2, -2.4, 21.1, 43.2), "gripper": rads(0)}
	test.That(t, planJump(head, next, arms, 10*time.Millisecond).tooFar, test.ShouldBeFalse)

	// A flip to another kinematic branch, as seen on an xArm6 for a 1mm hand move 176ms after
	// the previous plan, is refused.
	flipped := referenceframe.FrameSystemInputs{"arm": rads(45.7, -1.7, -22.3, 176.6, -19.2, -133.5), "gripper": rads(0)}
	res := planJump(head, flipped, arms, 176*time.Millisecond)
	test.That(t, res.tooFar, test.ShouldBeTrue)
	test.That(t, res.arm, test.ShouldEqual, "arm")
	test.That(t, res.degs, test.ShouldAlmostEqual, 179.1, 0.1)
	test.That(t, res.allowedDegs, test.ShouldBeLessThan, 40)

	// It is refused however long ago the last plan was.
	test.That(t, planJump(head, flipped, arms, time.Hour).tooFar, test.ShouldBeTrue)

	// A smaller discontinuity, 22 degrees 21ms after the previous plan, is refused too.
	hop := referenceframe.FrameSystemInputs{"arm": rads(42.9, -56.0, 8.0, -2.5, 20.9, 42.9), "gripper": rads(0)}
	test.That(t, planJump(head, hop, arms, 21*time.Millisecond).tooFar, test.ShouldBeTrue)

	// Catching up after a pause in the operator's poses is allowed: 30 degrees after 300ms.
	catchUp := referenceframe.FrameSystemInputs{"arm": rads(42.9, -4.0, 8.0, -2.5, 20.9, 42.9), "gripper": rads(0)}
	test.That(t, planJump(head, catchUp, arms, 300*time.Millisecond).tooFar, test.ShouldBeFalse)

	// Components that are not arms are not judged.
	gripperOnly := referenceframe.FrameSystemInputs{"arm": head["arm"], "gripper": rads(90)}
	test.That(t, planJump(head, gripperOnly, arms, time.Millisecond).tooFar, test.ShouldBeFalse)
}

func TestTeleopConfigValidate(t *testing.T) {
	valid := func(c Config) error {
		_, _, err := c.Validate("")
		return err
	}
	test.That(t, valid(Config{}), test.ShouldBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{ChaseGainPerSec: -1}}), test.ShouldNotBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{MaxVelDegsPerSec: -1}}), test.ShouldNotBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{MaxAccDegsPerSec2: -1}}), test.ShouldNotBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{
		MaxVelDegsPerSec: 120, MaxAccDegsPerSec2: 500, ChaseGainPerSec: 20,
	}}), test.ShouldBeNil)

	// The stream options teleop runs with are valid.
	opts := teleopStreamOptions()
	test.That(t, opts.Validate(), test.ShouldBeNil)
	test.That(t, opts.ArmSideTargetRunway, test.ShouldEqual, teleopStreamRunwayMs*time.Millisecond)
	test.That(t, opts.SendToArmInterval, test.ShouldEqual, teleopStreamIntervalMs*time.Millisecond)

	// The chaser gain defaults, and a configured value is used.
	test.That(t, TeleopStreamConfig{}.chase(), test.ShouldResemble, teleopChase{gain: defaultTeleopChaseGainPerSec})
	test.That(t, TeleopStreamConfig{ChaseGainPerSec: 20}.chase(), test.ShouldResemble, teleopChase{gain: 20})
	// A gain above the stable range is capped.
	test.That(t, TeleopStreamConfig{ChaseGainPerSec: 60}.chase(), test.ShouldResemble, teleopChase{gain: teleopChaseMaxGainPerSec})
}
