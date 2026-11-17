package builtin

import (
	"context"
	"path/filepath"
	"strings"
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
)

// fakeTeleopArmStreamRunner stands in for streaming.Run. It records the seed and every target
// it is handed, records a trajex runway of zero per target so the executor never skips, and
// returns when targets is closed, when ctx is cancelled, or with failErr right after the first
// target if failErr is set.
type fakeTeleopArmStreamRunner struct {
	mu            sync.Mutex
	runs          int
	returns       int
	closedCleanly int
	seeds         [][]referenceframe.Input
	targets       [][]referenceframe.Input
	failErr       error
}

func (f *fakeTeleopArmStreamRunner) run(
	ctx context.Context,
	_ logging.Logger,
	_ arm.Arm,
	_ streaming.StreamOptions,
	targets <-chan []referenceframe.Input,
	seed []referenceframe.Input,
	diag *diagnostics.SingleSessionDiagnostics,
	acks chan<- motion.TempStreamResponse,
) error {
	f.mu.Lock()
	f.runs++
	f.seeds = append(f.seeds, seed)
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
			diag.RecordTrajexRunway(0)
			runwayMs := int32(0)
			select {
			case acks <- motion.TempStreamResponse{MotionSideCurrentRunwayMs: &runwayMs}:
			case <-ctx.Done():
				return ctx.Err()
			}
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

func TestTeleopArmStream(t *testing.T) {
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
	for _, target := range state.targets {
		test.That(t, len(target), test.ShouldEqual, len(initial))
	}

	info := armStreamStatus(t, ms)
	test.That(t, info, test.ShouldNotBeNil)
	test.That(t, info["open"], test.ShouldBeTrue)
	test.That(t, info["error"], test.ShouldBeNil)

	// Teleop only reaches the arm through its stream, so the fake arm has not been teleported.
	after, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, after, test.ShouldResemble, initial)

	// Stopping closes the stream gracefully: the target channel is closed, not the context.
	teleopStop(t, ms)
	waitFor(t, "arm stream to drain and return", func() bool {
		return fake.snapshot().returns == 1
	})
	test.That(t, fake.snapshot().closedCleanly, test.ShouldEqual, 1)
	test.That(t, teleopStatus(t, ms)["running"], test.ShouldBeFalse)

	// The stream's diagnostics file is written when it ends.
	pattern := filepath.Join(diagDir, "tag=teleop-arm-stream-diagnostics", "tag="+teleopTestArm, "*.json")
	waitFor(t, "diagnostics file to be written", func() bool {
		files, err := filepath.Glob(pattern)
		test.That(t, err, test.ShouldBeNil)
		return len(files) == 1
	})
}

func TestTeleopStreamFailureIsReported(t *testing.T) {
	ms, a, teardown := setupTeleopTest(t)
	defer teardown()
	ctx := context.Background()

	fake := &fakeTeleopArmStreamRunner{failErr: status.Error(codes.Unimplemented, "no streaming here")}
	ms.teleopRunArmStream = fake.run

	initial, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	pose := teleopStart(t, ms)

	// Keep nudging the goal until the executor has seen the stream die.
	seq := 0
	waitFor(t, "executor to report the dead stream", func() bool {
		seq++
		teleopMove(t, ms, offsetPose(pose, float64(seq)), seq)
		info := armStreamStatus(t, ms)
		return info != nil && info["error"] != nil
	})
	info := armStreamStatus(t, ms)
	test.That(t, info["error"], test.ShouldContainSubstring, "no streaming here")
	test.That(t, info["open"], test.ShouldBeFalse)

	// The failure stays reported, and the stream is not reopened on later trajectories.
	plansAtFailure := teleopStatus(t, ms)["plan_count"].(int64)
	waitFor(t, "more plans after the failure", func() bool {
		seq++
		teleopMove(t, ms, offsetPose(pose, float64(seq)), seq)
		return teleopStatus(t, ms)["plan_count"].(int64) > plansAtFailure+3
	})
	waitFor(t, "pipeline to report the stream error", func() bool {
		seq++
		teleopMove(t, ms, offsetPose(pose, float64(seq)), seq)
		pipelineErr, _ := teleopStatus(t, ms)["error"].(string)
		return strings.Contains(pipelineErr, "no streaming here")
	})
	test.That(t, fake.snapshot().runs, test.ShouldEqual, 1)

	// Nothing else moves the arm: there is no other path to it.
	after, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, after, test.ShouldResemble, initial)

	teleopStop(t, ms)

	// A new session starts clean and opens a stream again.
	teleopStart(t, ms)
	teleopMove(t, ms, offsetPose(pose, 50), 1)
	waitFor(t, "a new session to open a stream", func() bool {
		return fake.snapshot().runs == 2
	})
	teleopStop(t, ms)
}

func TestTeleopConfigValidate(t *testing.T) {
	valid := func(c Config) error {
		_, _, err := c.Validate("")
		return err
	}
	test.That(t, valid(Config{}), test.ShouldBeNil)

	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{MaxTrajexLeadMs: -1}}), test.ShouldNotBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{ArmSideTargetRunwayMs: 10, SendToArmIntervalMs: 10}}), test.ShouldNotBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{MaxVelDegsPerSecJoints: []float64{90, -1}}}), test.ShouldNotBeNil)
	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{
		ArmSideTargetRunwayMs: 40, SendToArmIntervalMs: 10, MaxVelDegsPerSec: 120, MaxAccDegsPerSec2: 500, MaxTrajexLeadMs: 60,
	}}), test.ShouldBeNil)

	// Defaults land in the stream options teleop will run with.
	opts := TeleopStreamConfig{}.streamOptions()
	test.That(t, opts.ArmSideTargetRunway, test.ShouldEqual, defaultTeleopStreamRunwayMs*time.Millisecond)
	test.That(t, opts.SendToArmInterval, test.ShouldEqual, defaultTeleopStreamIntervalMs*time.Millisecond)
	test.That(t, opts.MoveOptions.MaxVelRads, test.ShouldAlmostEqual, utils.DegToRad(defaultTeleopStreamMaxVelDegsPerSec), 1e-9)
	test.That(t, TeleopStreamConfig{}.maxLead(), test.ShouldEqual, defaultTeleopStreamMaxLeadMs*time.Millisecond)

	test.That(t, valid(Config{TeleopStream: TeleopStreamConfig{MaxTargetsDroppedInARow: -1}}), test.ShouldNotBeNil)
	opts = TeleopStreamConfig{MaxTargetsDroppedInARow: 3, ForceRebaseIfExtendLatencyExceedsMs: 5}.streamOptions()
	test.That(t, opts.MaxTargetsDroppedInARowDueToTrajexExtendError, test.ShouldEqual, 3)
	test.That(t, opts.ForceRebaseIfExtendLatencyExceeds, test.ShouldEqual, 5*time.Millisecond)
}
