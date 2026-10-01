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
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/testutils/inject"
)

// newStreamTestService builds a minimal builtIn wired to a single injected arm
// that records the trajectory points it receives over the streamed RPC.
func newStreamTestService(t *testing.T) (*builtIn, func() (points, streams, stops int)) {
	t.Helper()
	var mu sync.Mutex
	var points, streams, stops int

	inj := inject.NewArm("arm")
	inj.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return make([]referenceframe.Input, 6), nil
	}
	inj.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		mu.Lock()
		stops++
		mu.Unlock()
		return nil
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
	return ms, func() (int, int, int) {
		mu.Lock()
		defer mu.Unlock()
		return points, streams, stops
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

	// status agrees the session has ended, but diagnostics from it are still retained (see
	// TestTempStreamArmJointPositionsRetainsDiagnosticsAfterSessionEnds)
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)

	// The session did not end vacuously: trajectory points reached the arm over a
	// stream RPC.
	points, streams, stops := counts()
	// A clean drain waits for the arm to finish on its own; nothing stops it.
	test.That(t, stops, test.ShouldEqual, 0)
	test.That(t, points > 0, test.ShouldBeTrue)
	test.That(t, streams >= 1, test.ShouldBeTrue)
}

// TestTempStreamArmJointPositionsAcksEachTarget checks that the builtin acknowledges every
// target it admits, one response per target, which is what lets a client pace itself by
// execution instead of by the transport's buffering.
func TestTempStreamArmJointPositionsAcksEachTarget(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	targets := make(chan []referenceframe.Input)
	responses := make(chan motion.TempStreamResponse)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), targets, responses, nil)
	}()
	waitForSession(t, ms, "arm")

	const n = 6
	for i := 1; i <= n; i++ {
		targets <- []referenceframe.Input{float64(i) * 0.02, 0, 0, 0, 0, 0}
		select {
		case <-responses:
		case <-time.After(10 * time.Second):
			t.Fatalf("no acknowledgment for target %d", i)
		}
	}

	close(targets)
	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("TempStreamArmJointPositions never returned")
	}
	// Nothing was acknowledged beyond the n targets.
	select {
	case <-responses:
		t.Fatal("unexpected acknowledgment after the drain")
	default:
	}
	close(responses)
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

// TestTempStreamArmJointPositionsRetainsDiagnosticsAfterSessionEnds checks that a finished
// session's diagnostics remain visible over stream_status: running reports false, but
// last_window_details still holds what that session recorded, rather than the response going
// empty the moment the session ends.
func TestTempStreamArmJointPositionsRetainsDiagnosticsAfterSessionEnds(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	opts := streamTestOptions()
	diagWindow := int32(60)
	opts.DiagnosticsWindowSecs = &diagWindow
	targets := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.TempStreamArmJointPositions(ctx, "arm", opts, targets, ignoredResponses(t), nil)
	}()
	waitForSession(t, ms, "arm")
	targets <- []referenceframe.Input{0.1, 0, 0, 0, 0, 0}
	close(targets)
	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("TempStreamArmJointPositions never returned")
	}

	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)
	test.That(t, resp["diagnostics_window_secs"], test.ShouldEqual, 60)
	details, ok := resp["last_window_details"].(diagnostics.SingleSessionLastWindowDetails)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(details.JointPositionTargetReceived), test.ShouldEqual, 1)
}

// TestTempStreamArmJointPositionsSecondSessionAppendsToRetainedWindow checks that a new session
// on an arm that already streamed continues appending to the same retained window instead of
// starting over: the first session's recorded events are still present once the second session
// has recorded its own.
func TestTempStreamArmJointPositionsSecondSessionAppendsToRetainedWindow(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	opts := streamTestOptions()
	diagWindow := int32(60)
	opts.DiagnosticsWindowSecs = &diagWindow

	runOneTarget := func() {
		targets := make(chan []referenceframe.Input)
		errCh := make(chan error, 1)
		go func() {
			errCh <- ms.TempStreamArmJointPositions(ctx, "arm", opts, targets, ignoredResponses(t), nil)
		}()
		waitForSession(t, ms, "arm")
		targets <- []referenceframe.Input{0.1, 0, 0, 0, 0, 0}
		close(targets)
		select {
		case err := <-errCh:
			test.That(t, err, test.ShouldBeNil)
		case <-time.After(10 * time.Second):
			t.Fatal("TempStreamArmJointPositions never returned")
		}
	}

	runOneTarget()
	runOneTarget()

	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldBeNil)
	details, ok := resp["last_window_details"].(diagnostics.SingleSessionLastWindowDetails)
	test.That(t, ok, test.ShouldBeTrue)
	// Both sessions' targets are present: two, not one.
	test.That(t, len(details.JointPositionTargetReceived), test.ShouldEqual, 2)
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
	stopCalled := make(chan struct{}, 1)
	inj.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		select {
		case stopCalled <- struct{}{}:
		default:
		}
		return nil
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
		if resp["running"] == false {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("aborted session never finished tearing down")
		}
		time.Sleep(10 * time.Millisecond)
	}
	test.That(t, <-errCh, test.ShouldNotBeNil)
	// An abort stops the arm explicitly, since the cancelled arm RPC does not wait for it.
	test.That(t, len(stopCalled), test.ShouldEqual, 1)
	newTargets := make(chan []referenceframe.Input)
	close(newTargets)
	err = ms.TempStreamArmJointPositions(ctx, "arm", streamTestOptions(), newTargets, ignoredResponses(t), nil)
	test.That(t, err, test.ShouldBeNil)
}
