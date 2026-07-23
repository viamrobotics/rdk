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

func streamTestConfig() map[string]interface{} {
	return map[string]interface{}{
		"buffer_ahead_in_arm_ms":   50,
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
		DoStreamStart: map[string]interface{}{"arm": "arm", "config": streamTestConfig()},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[DoStreamStart], test.ShouldEqual, true)

	// starting again while running should error
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: "arm"})
	test.That(t, err, test.ShouldNotBeNil)

	// push a ramp on joint 0
	for i := 1; i <= 6; i++ {
		resp, err = ms.DoCommand(ctx, map[string]interface{}{
			DoStreamPush: []interface{}{float64(i) * 0.02, 0.0, 0.0, 0.0, 0.0, 0.0},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp[DoStreamPush], test.ShouldEqual, 1)
	}

	// status: running
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	status, ok := resp[DoStreamStatus].(map[string]any)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, status["running"], test.ShouldEqual, true)
	test.That(t, status["arm"], test.ShouldEqual, "arm")

	// stop gracefully
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStop: true})
	test.That(t, err, test.ShouldBeNil)
	stopStatus, ok := resp[DoStreamStop].(map[string]any)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, stopStatus["running"], test.ShouldEqual, false)
	_, hasErr := stopStatus["error"]
	test.That(t, hasErr, test.ShouldBeFalse)

	points, streams := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
	test.That(t, streams >= 1, test.ShouldBeTrue)

	// status after stop: not running
	resp, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	status, _ = resp[DoStreamStatus].(map[string]any)
	test.That(t, status["running"], test.ShouldEqual, false)
}

func TestDoCommandArmStreamingErrors(t *testing.T) {
	ms, _ := newStreamTestService(t)
	ctx := context.Background()

	// push before start
	_, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamPush: []interface{}{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}})
	test.That(t, err, test.ShouldNotBeNil)

	// start referencing an unknown arm
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: "nope"})
	test.That(t, err, test.ShouldNotBeNil)

	// start missing an arm name
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStart: map[string]interface{}{"config": streamTestConfig()}})
	test.That(t, err, test.ShouldNotBeNil)

	// status with no session is not running
	resp, err := ms.DoCommand(ctx, map[string]interface{}{DoStreamStatus: true})
	test.That(t, err, test.ShouldBeNil)
	status, _ := resp[DoStreamStatus].(map[string]any)
	test.That(t, status["running"], test.ShouldEqual, false)
}

func TestDoCommandArmStreamingBatch(t *testing.T) {
	ms, counts := newStreamTestService(t)
	defer func() { test.That(t, ms.Close(context.Background()), test.ShouldBeNil) }()
	ctx := context.Background()

	_, err := ms.DoCommand(ctx, map[string]interface{}{
		DoStreamStart: map[string]interface{}{"arm": "arm", "config": streamTestConfig()},
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
	test.That(t, resp[DoStreamPush], test.ShouldEqual, 2)

	// keep pushing
	_, err = ms.DoCommand(ctx, map[string]interface{}{
		DoStreamPush: []interface{}{0.06, 0.0, 0.0, 0.0, 0.0, 0.0},
	})
	test.That(t, err, test.ShouldBeNil)

	// give the pipeline a moment, then stop gracefully
	time.Sleep(50 * time.Millisecond)
	_, err = ms.DoCommand(ctx, map[string]interface{}{DoStreamStop: true})
	test.That(t, err, test.ShouldBeNil)

	points, _ := counts()
	test.That(t, points > 0, test.ShouldBeTrue)
}
