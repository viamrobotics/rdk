//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package builtin

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func testKinematics(dof int, velRadPerSec, accelRadPerSec2 float64) (referenceframe.Model, error) {
	limit := referenceframe.Limit{
		Min:             -math.Pi,
		Max:             math.Pi,
		MaxVelocity:     &velRadPerSec,
		MaxAcceleration: &accelRadPerSec2,
	}

	fs := referenceframe.NewEmptyFrameSystem("test")
	parent := fs.World()
	var last referenceframe.Frame
	for i := range dof {
		f, err := referenceframe.NewRotationalFrame(fmt.Sprintf("j%d", i), spatialmath.R4AA{RZ: 1}, limit)
		if err != nil {
			return nil, err
		}
		if err := fs.AddFrame(f, parent); err != nil {
			return nil, err
		}
		parent = f
		last = f
	}
	return referenceframe.NewModel("test", fs, last.Name())
}

// newStreamTestService builds a minimal builtIn wired to a single injected arm
// that records the trajectory points it receives over the streamed RPC.
func newStreamTestService(t *testing.T) (*builtIn, func() (points, streams int)) {
	t.Helper()
	var mu sync.Mutex
	var points, streams int

	inj := inject.NewArm("arm")
	inj.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return make([]referenceframe.Input, 6), nil
	}
	inj.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
		return testKinematics(6, math.Pi/6, math.Pi/3)
	}
	inj.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context,
		batches <-chan []arm.TrajectoryPoint,
		responses chan<- arm.Response,
		extra map[string]interface{},
	) error {
		mu.Lock()
		streams++
		mu.Unlock()
		for batch := range batches {
			mu.Lock()
			points += len(batch)
			mu.Unlock()
		}
		return nil
	}

	ms := &builtIn{
		logger:     logging.NewTestLogger(t),
		components: map[string]resource.Resource{"arm": inj},
	}
	return ms, func() (int, int) {
		mu.Lock()
		defer mu.Unlock()
		return points, streams
	}
}

func streamTestOptions() motion.StreamOptions {
	runway := int32(50)
	interval := int32(10)
	return motion.StreamOptions{
		TargetRunwayInArmMs: &runway,
		SendToArmIntervalMs: &interval,
	}
}

// runStream runs StreamArmJointPositions in the background, feeding it the given waypoints
// (in order) and then closing targets. It returns a channel that receives the call's error once
// it returns.
func runStream(ctx context.Context, ms *builtIn, armName string, opts motion.StreamOptions, waypoints [][]referenceframe.Input) <-chan error {
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.StreamArmJointPositions(ctx, armName, opts, targets, nil)
	}()
	go func() {
		defer close(targets)
		for _, wp := range waypoints {
			select {
			case targets <- wp:
			case <-ctx.Done():
				return
			}
		}
	}()
	return errCh
}

func TestStreamArmJointPositionsHappyPath(t *testing.T) {
	ms, counts := newStreamTestService(t)
	ctx := context.Background()

	// status before start shows that no session is running
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)

	waypoints := make([][]referenceframe.Input, 6)
	for i := range waypoints {
		waypoints[i] = []referenceframe.Input{float64(i+1) * 0.02, 0, 0, 0, 0, 0}
	}

	// runStream blocks until targets is closed and the derived trajectory finishes, so run it in
	// the background and wait on its error with a generous deadline: if it hung, the test would
	// fail on the deadline rather than hanging forever.
	errCh := runStream(ctx, ms, "arm", streamTestOptions(), waypoints)

	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("StreamArmJointPositions never returned")
	}

	// status agrees the session has ended
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)
	_, hasErr := resp["error"]
	test.That(t, hasErr, test.ShouldBeFalse)

	// The session did not end vacuously: trajectory points reached the arm over a stream RPC.
	points, streams := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
	test.That(t, streams >= 1, test.ShouldBeTrue)
}

// TestStreamArmJointPositionsStatusDiagnosticsOptIn checks that stream_status omits the
// (potentially large) last window details unless the caller opts in via
// {"last_window_details": true}.
func TestStreamArmJointPositionsStatusDiagnosticsOptIn(t *testing.T) {
	ms, _ := newStreamTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := streamTestOptions()
	diagWindow := int32(60_000)
	opts.DiagnosticsWindowMs = &diagWindow
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() { errCh <- ms.StreamArmJointPositions(ctx, "arm", opts, targets, nil) }()

	// Give the session a moment to register before polling status.
	waitForRunning(t, ms, "arm")

	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	_, hasDetails := resp["last_window_details"]
	test.That(t, hasDetails, test.ShouldBeFalse)
	test.That(t, resp["running"], test.ShouldEqual, true)

	resp, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStatus: map[string]interface{}{"arm": "arm", "last_window_details": true},
	})
	test.That(t, err, test.ShouldBeNil)
	_, hasDetails = resp["last_window_details"]
	test.That(t, hasDetails, test.ShouldBeTrue)

	close(targets)
	<-errCh
}

// TestStreamArmJointPositionsDiagnosticsDisabled checks that diagnostics_window_ms: 0 starts a
// session with no diagnostics, so opting in to last_window_details yields nothing.
func TestStreamArmJointPositionsDiagnosticsDisabled(t *testing.T) {
	ms, _ := newStreamTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := streamTestOptions()
	diagWindow := int32(0)
	opts.DiagnosticsWindowMs = &diagWindow
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() { errCh <- ms.StreamArmJointPositions(ctx, "arm", opts, targets, nil) }()

	waitForRunning(t, ms, "arm")

	resp, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStatus: map[string]interface{}{"arm": "arm", "last_window_details": true},
	})
	test.That(t, err, test.ShouldBeNil)
	_, hasDetails := resp["last_window_details"]
	test.That(t, hasDetails, test.ShouldBeFalse)
	test.That(t, resp["running"], test.ShouldEqual, true)

	close(targets)
	<-errCh
}

func TestStreamArmJointPositionsUsedIncorrectly(t *testing.T) {
	ms, _ := newStreamTestService(t)
	ctx := context.Background()

	// unknown arm
	targets := make(chan []referenceframe.Input)
	close(targets)
	err := ms.StreamArmJointPositions(ctx, "nope", streamTestOptions(), targets, nil)
	test.That(t, err, test.ShouldNotBeNil)

	// invalid options fail synchronously rather than spawning a dead session
	badOpts := streamTestOptions()
	negativeInterval := int32(-5)
	badOpts.SendToArmIntervalMs = &negativeInterval
	err = ms.StreamArmJointPositions(ctx, "arm", badOpts, targets, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid streaming options")

	// starting again while one is running should error
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	running := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() { errCh <- ms.StreamArmJointPositions(runCtx, "arm", streamTestOptions(), running, nil) }()
	waitForRunning(t, ms, "arm")

	concurrentTargets := make(chan []referenceframe.Input)
	close(concurrentTargets)
	err = ms.StreamArmJointPositions(context.Background(), "arm", streamTestOptions(), concurrentTargets, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already running")

	close(running)
	<-errCh
}

func TestStreamArmJointPositionsAbortViaContextCancel(t *testing.T) {
	var rpcStarted sync.Once
	rpcStartedCh := make(chan struct{})
	releaseRPC := make(chan struct{})

	inj := inject.NewArm("arm")
	inj.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return make([]referenceframe.Input, 6), nil
	}
	inj.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
		return testKinematics(6, math.Pi/6, math.Pi/3)
	}
	inj.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context,
		batches <-chan []arm.TrajectoryPoint,
		responses chan<- arm.Response,
		extra map[string]interface{},
	) error {
		rpcStarted.Do(func() { close(rpcStartedCh) })
		// Simulate an arm impl that ignores ctx and blocks: teardown cannot finish until the
		// arm RPC returns.
		<-releaseRPC
		for range batches {
		}
		return nil
	}

	ms := &builtIn{
		logger:     logging.NewTestLogger(t),
		components: map[string]resource.Resource{"arm": inj},
	}

	runCtx, cancel := context.WithCancel(context.Background())
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() { errCh <- ms.StreamArmJointPositions(runCtx, "arm", streamTestOptions(), targets, nil) }()

	select {
	case <-rpcStartedCh:
	case <-time.After(10 * time.Second):
		t.Fatal("arm RPC never started")
	}

	// Cancel the caller's ctx: this is the new "abort."
	cancel()

	// A new session for the same arm fails while the aborted one is still tearing down (it is
	// blocked on the arm RPC, which we haven't released yet).
	blockedTargets := make(chan []referenceframe.Input)
	close(blockedTargets)
	err := ms.StreamArmJointPositions(context.Background(), "arm", streamTestOptions(), blockedTargets, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already running")

	// Let the RPC (and therefore teardown) finish.
	close(releaseRPC)
	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldNotBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("aborted session never finished tearing down")
	}
	_ = targets

	// Once the session has finished, a new one can start.
	newTargets := make(chan []referenceframe.Input)
	close(newTargets)
	test.That(t, ms.StreamArmJointPositions(context.Background(), "arm", streamTestOptions(), newTargets, nil), test.ShouldBeNil)
}

// waitForRunning polls stream_status until it reports the named arm's session as running, or
// fails the test after a generous deadline.
func waitForRunning(t *testing.T, ms *builtIn, armName string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := ms.DoCommand(context.Background(), map[string]interface{}{
			DoStreamStatus: map[string]interface{}{"arm": armName},
		})
		test.That(t, err, test.ShouldBeNil)
		if resp["running"] == true {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("session for %q never reported running", armName)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
