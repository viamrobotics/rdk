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

func streamTestOptions() map[string]interface{} {
	return map[string]interface{}{
		"target_runway_in_arm_ms":  50,
		"send_to_arm_interval_ms":  10,
		"vel_limit_deg_per_sec":    30,
		"accel_limit_deg_per_sec2": 60,
	}
}

func TestDoCommandArmStreaming(t *testing.T) {
	ms, counts := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	// start
	resp, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["ok"], test.ShouldEqual, 1)

	// starting again while running should error
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldNotBeNil)

	// push a ramp on joint 0
	for i := 1; i <= 6; i++ {
		resp, err = ms.DoCommand(ctx, map[string]interface{}{
			DoStreamPush: []interface{}{[]interface{}{float64(i) * 0.02, 0.0, 0.0, 0.0, 0.0, 0.0}},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["ok"], test.ShouldEqual, 1)
	}

	// status: running
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, true)
	test.That(t, resp["arm"], test.ShouldEqual, "arm")

	// flush blocks (ctx has no deadline) until the drain completes
	flushCtx, flushCancel := context.WithTimeout(ctx, 10*time.Second)
	defer flushCancel()
	resp, err = ms.DoCommand(flushCtx, map[string]interface{}{DoStreamFlush: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)
	_, hasErr := resp["error"]
	test.That(t, hasErr, test.ShouldBeFalse)

	// status agrees the session has ended
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)

	// The flush drained the full trajectory to the arm.
	points, streams := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
	test.That(t, streams >= 1, test.ShouldBeTrue)
}

// TestDoCommandArmStreamingStatusTraceOptOut checks that stream_status includes the trace
// snapshot by default (bare true, or the key present with no value), but omits it when the
// caller explicitly opts out via {"trace": false} -- the cheap-poll path a client should use
// while it only cares about "running"/"error", saving it from re-fetching and re-serializing
// the whole accumulated trace on every poll.
func TestDoCommandArmStreamingStatusTraceOptOut(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	_, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)

	// default (bare true): trace included
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	_, hasTrace := resp["trace"]
	test.That(t, hasTrace, test.ShouldBeTrue)

	// explicit opt-in: trace included
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"trace": true}})
	test.That(t, err, test.ShouldBeNil)
	_, hasTrace = resp["trace"]
	test.That(t, hasTrace, test.ShouldBeTrue)

	// explicit opt-out: trace omitted, but running/arm are still reported
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: map[string]interface{}{"trace": false}})
	test.That(t, err, test.ShouldBeNil)
	_, hasTrace = resp["trace"]
	test.That(t, hasTrace, test.ShouldBeFalse)
	test.That(t, resp["running"], test.ShouldEqual, true)
	test.That(t, resp["arm"], test.ShouldEqual, "arm")

	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamAbort: true})
	test.That(t, err, test.ShouldBeNil)
}

func TestDoCommandArmStreamingErrors(t *testing.T) {
	ms, _ := newStreamTestService(t)
	ctx := context.Background()

	// push before start
	_, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamPush: []interface{}{[]interface{}{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}})
	test.That(t, err, test.ShouldNotBeNil)

	// start referencing an unknown arm
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: map[string]interface{}{"arm": "nope"}})
	test.That(t, err, test.ShouldNotBeNil)

	// start missing an arm name
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: map[string]interface{}{"options": streamTestOptions()}})
	test.That(t, err, test.ShouldNotBeNil)

	// start with invalid options fails synchronously rather than spawning a dead session
	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{
			"arm":     "arm",
			"options": map[string]interface{}{"send_to_arm_interval_ms": -5},
		},
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid streaming options")

	// status with no session is not running
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)
}

// TestDoCommandArmStreamingAbortTeardown checks that when an abort's request context expires
// before session teardown completes, the session stays registered: status and abort report
// running, a new start refuses to race a second stream onto the arm, and once teardown finishes
// a new session can start.
func TestDoCommandArmStreamingAbortTeardown(t *testing.T) {
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

	_, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)

	// Push until the executor opens the arm RPC.
	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{[]interface{}{0.2, 0.0, 0.0, 0.0, 0.0, 0.0}},
	})
	test.That(t, err, test.ShouldBeNil)
	select {
	case <-rpcStartedCh:
	case <-time.After(10 * time.Second):
		t.Fatal("arm RPC never started")
	}

	// Abort with an already-expired context: teardown is blocked on the arm RPC,
	// so the session stays registered and reports running.
	expiredCtx, cancel := context.WithCancel(ctx)
	cancel()
	status := ms.streamAbort(expiredCtx)
	test.That(t, status["running"], test.ShouldEqual, true)

	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, true)

	// A new start must not race a second stream onto the arm while the aborted
	// session is still tearing down.
	_, err = ms.DoCommand(expiredCtx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "still shutting down")

	// Let the RPC (and therefore teardown) finish. Once status reports the
	// session ended, a new session can start.
	close(releaseRPC)
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
		test.That(t, err, test.ShouldBeNil)
		if resp["running"] == false {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("aborted session never finished tearing down")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)
}

// TestDoCommandArmStreamingProtocolEnforcement checks that the single-controller push protocol
// is enforced explicitly: a push overlapping another in-flight push or flush is rejected with a
// protocol error rather than interleaved, and a push after flush is rejected rather than parked.
func TestDoCommandArmStreamingProtocolEnforcement(t *testing.T) {
	ms, _ := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	_, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)

	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{[]interface{}{0.02, 0.0, 0.0, 0.0, 0.0, 0.0}},
	})
	test.That(t, err, test.ShouldBeNil)

	// A push while another operation holds the protocol lock is rejected, not queued.
	ms.streamMu.RLock()
	s := ms.stream
	ms.streamMu.RUnlock()
	s.opMu.Lock()
	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{[]interface{}{0.04, 0.0, 0.0, 0.0, 0.0, 0.0}},
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "overlapping stream operations")
	s.opMu.Unlock()

	// Flush, then push: rejected with a clear error rather than blocking until session end.
	flushCtx, flushCancel := context.WithTimeout(ctx, 10*time.Second)
	defer flushCancel()
	_, err = ms.DoCommand(flushCtx, map[string]interface{}{DoStreamFlush: true})
	test.That(t, err, test.ShouldBeNil)

	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{[]interface{}{0.06, 0.0, 0.0, 0.0, 0.0, 0.0}},
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "flushing")
}

func TestDoCommandArmStreamingBatch(t *testing.T) {
	ms, counts := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	_, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)

	// push a batch of two waypoints at once
	resp, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{
			[]interface{}{0.02, 0.0, 0.0, 0.0, 0.0, 0.0},
			[]interface{}{0.04, 0.0, 0.0, 0.0, 0.0, 0.0},
		},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["ok"], test.ShouldEqual, 1)

	// keep pushing
	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{[]interface{}{0.06, 0.0, 0.0, 0.0, 0.0, 0.0}},
	})
	test.That(t, err, test.ShouldBeNil)

	// give the pipeline a moment to deliver the first batch to the arm, then stop
	time.Sleep(100 * time.Millisecond)
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamAbort: true})
	test.That(t, err, test.ShouldBeNil)

	points, _ := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
}
