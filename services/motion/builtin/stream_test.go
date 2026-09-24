//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package builtin

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/testutils/inject"
)

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

func streamTestOptions() motion.TempStreamOptions {
	runway, interval := int32(50), int32(10)
	return motion.TempStreamOptions{
		ArmSideTargetRunwayMs: &runway,
		SendToArmIntervalMs:   &interval,
	}
}

// ignoredResponses returns a responses channel whose acknowledgments are discarded in the
// background until the test ends, for calls whose acknowledgments are not under test.
func ignoredResponses(t *testing.T) chan motion.TempStreamResponse {
	t.Helper()
	responses := make(chan motion.TempStreamResponse)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range responses {
		}
	}()
	t.Cleanup(func() {
		close(responses)
		<-done
	})
	return responses
}

// waitForSession polls until the named arm's session is registered, or fails the test after a
// generous deadline.
func waitForSession(t *testing.T, ms *builtIn, armName string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		ms.streamMu.RLock()
		_, ok := ms.streams[armName]
		ms.streamMu.RUnlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("session for %q never registered", armName)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTempStreamArmJointPositionsHappyPath(t *testing.T) {
	ms, counts := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	// status before start shows that no session is running
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldBeEmpty)

	// start
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), targets, ignoredResponses(t), nil)
	}()
	waitForSession(t, ms, "arm")

	// push a ramp on joint 0
	for i := 1; i <= 6; i++ {
		targets <- []referenceframe.Input{float64(i) * 0.02, 0, 0, 0, 0, 0}
	}

	// status: running
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeEmpty)

	// flush: closing targets drains what's queued, and the call returns once the drain completes.
	// The deadline is a backstop that turns a hung drain into a test failure rather than a stuck
	// test.
	close(targets)
	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("TempStreamArmJointPositions never returned")
	}

	// status agrees the session has ended
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldBeEmpty)

	// The session did not end vacuously: trajectory points reached the arm over a
	// stream RPC.
	points, streams := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
	test.That(t, streams >= 1, test.ShouldBeTrue)
}

// TestTempStreamArmJointPositionsStatusReturnsDetails checks that stream_status carries a
// running session's last window details whenever a positive diagnostics window retains them.
//
//nolint:dupl
func TestTempStreamArmJointPositionsStatusReturnsDetails(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := streamTestOptions()
	diagWindow := int32(60)
	opts.DiagnosticsWindowSecs = &diagWindow
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(ctx, "arm", opts, targets, ignoredResponses(t), nil)
	}()
	waitForSession(t, ms, "arm")

	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["diagnostics_window_secs"], test.ShouldEqual, 60)
	_, hasDetails := resp["last_window_details"]
	test.That(t, hasDetails, test.ShouldBeTrue)

	// abort
	cancel()
	<-errCh
}

// TestTempStreamArmJointPositionsDiagnosticsDisabled checks that diagnostics_window_secs: 0
// disables retention of last_window_details.
//
//nolint:dupl
func TestTempStreamArmJointPositionsDiagnosticsDisabled(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := streamTestOptions()
	diagWindow := int32(0)
	opts.DiagnosticsWindowSecs = &diagWindow
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(ctx, "arm", opts, targets, ignoredResponses(t), nil)
	}()
	waitForSession(t, ms, "arm")

	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["diagnostics_window_secs"], test.ShouldEqual, 0)
	_, hasDetails := resp["last_window_details"]
	test.That(t, hasDetails, test.ShouldBeFalse)

	// abort
	cancel()
	<-errCh
}

func TestTempStreamArmJointPositionsUsedIncorrectly(t *testing.T) {
	ms, _ := newStreamTestService(t)
	ctx := context.Background()

	targets := make(chan []referenceframe.Input)
	close(targets)

	// start referencing an unknown arm
	err := ms.TempStreamArmJointPositions(ctx, "nope", streamTestOptions(), targets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldNotBeNil)

	// start missing an arm name
	err = ms.TempStreamArmJointPositions(ctx, "", streamTestOptions(), targets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldNotBeNil)

	// start with invalid options fails synchronously rather than spawning a dead session
	badOpts := streamTestOptions()
	negativeInterval := int32(-5)
	badOpts.SendToArmIntervalMs = &negativeInterval
	err = ms.TempStreamArmJointPositions(ctx, "arm", badOpts, targets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid streaming options")

	// starting again while running should error
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	running := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), running, ignoredResponses(t), nil)
	}()
	waitForSession(t, ms, "arm")
	err = ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), targets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already running")

	close(running)
	<-errCh
}

func TestTempStreamArmJointPositionsAbort(t *testing.T) {
	var rpcStarted sync.Once
	rpcStartedCh := make(chan struct{})
	releaseRPC := make(chan struct{})

	inj := inject.NewArm("arm")
	inj.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return make([]referenceframe.Input, 6), nil
	}
	inj.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context,
		batches <-chan []arm.TrajectoryPoint,
		responses chan<- arm.Response,
		extra map[string]interface{},
	) error {
		rpcStarted.Do(func() { close(rpcStartedCh) })
		// Simulate an arm impl that ignores ctx and blocks: teardown cannot
		// finish until the RPC returns.
		<-releaseRPC
		for range batches {
		}
		return nil
	}

	ms := &builtIn{
		logger:     logging.NewTestLogger(t),
		components: map[string]resource.Resource{"arm": inj},
	}
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(runCtx, "arm", streamTestOptions(), targets, ignoredResponses(t), nil)
	}()
	waitForSession(t, ms, "arm")

	targets <- []referenceframe.Input{0.2, 0, 0, 0, 0, 0}

	// Wait for the arm RPC to be open (and parked on releaseRPC) before aborting.
	select {
	case <-rpcStartedCh:
	case <-time.After(10 * time.Second):
		t.Fatal("arm RPC never started")
	}

	// Abort by canceling the caller's ctx. Teardown is blocked on the arm RPC, so the session
	// stays registered and status still reports it.
	cancel()
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeEmpty)

	// A new session should fail because the previous one is still running.
	blockedTargets := make(chan []referenceframe.Input)
	close(blockedTargets)
	err = ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), blockedTargets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already running")

	// Let the RPC (and therefore teardown) finish. Once status reports the
	// session ended, a new session can start.
	close(releaseRPC)
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
		test.That(t, err, test.ShouldBeNil)
		if len(resp) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("aborted session never finished tearing down")
		}
		time.Sleep(10 * time.Millisecond)
	}
	test.That(t, <-errCh, test.ShouldNotBeNil)
	newTargets := make(chan []referenceframe.Input)
	close(newTargets)
	err = ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), newTargets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldBeNil)
}
