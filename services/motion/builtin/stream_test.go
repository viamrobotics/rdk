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

func TestDoCommandsHappyPath(t *testing.T) {
	ms, counts := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	// status before start shows that no session is running
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["running"], test.ShouldEqual, false)

	// start
	resp, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["ok"], test.ShouldEqual, 1)

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

	// flush blocks until the drain completes. The deadline is a backstop: if it fired
	// first, flush would report running=true and the assertions below would fail,
	// turning a hung drain into a test failure rather than a stuck test.
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

	// The session did not end vacuously: trajectory points reached the arm over a
	// stream RPC.
	points, streams := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
	test.That(t, streams >= 1, test.ShouldBeTrue)
}

func TestDoCommandsUsedIncorrectly(t *testing.T) {
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

	// starting again while running should error
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	resp, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "options": streamTestOptions()},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["ok"], test.ShouldEqual, 1)
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: map[string]interface{}{"arm": "arm"}})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestDoCommandStreamAbort(t *testing.T) {
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

	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{[]interface{}{0.2, 0.0, 0.0, 0.0, 0.0, 0.0}},
	})
	test.That(t, err, test.ShouldBeNil)

	// Wait for the arm RPC to be open (and parked on releaseRPC) before aborting.
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

	// A new startStream should fail because the previous session is still running.
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

func TestDoCommandSequentialCommandsEnforcement(t *testing.T) {
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

func TestDoCommandStreamPushBatch(t *testing.T) {
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

	// Wait until the arm has received at least one trajectory point.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if points, _ := counts(); points > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("arm never received trajectory points")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
