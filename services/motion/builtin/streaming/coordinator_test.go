// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/testutils/inject"
)

func TestStreamOptionsApplyDefaultsAndValidate(t *testing.T) {
	c := StreamOptions{}
	c.ApplyDefaults()
	test.That(t, c.TargetRunwayInArmMs, test.ShouldEqual, defaultTargetRunwayInArmMs)
	test.That(t, c.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, c.VelLimitDegPerSec, test.ShouldEqual, defaultVelLimitDegPerSec)
	test.That(t, c.AccelLimitDegPerSec2, test.ShouldEqual, defaultAccelLimitDegPerSec2)
	test.That(t, c.MaxRunwayInSessionMs, test.ShouldEqual, defaultMaxRunwayInSessionMs)
	test.That(t, c.Validate(), test.ShouldBeNil)

	test.That(t, (&StreamOptions{SendToArmIntervalMs: 10, TargetRunwayInArmMs: -1}).Validate(), test.ShouldNotBeNil)
	// A zero send interval is invalid (division by zero when converting to Hz).
	test.That(t, (&StreamOptions{SendToArmIntervalMs: 0, TargetRunwayInArmMs: 10}).Validate(), test.ShouldNotBeNil)
	test.That(t, (&StreamOptions{SendToArmIntervalMs: 10, VelLimitDegPerSec: -1}).Validate(), test.ShouldNotBeNil)
	test.That(t, (&StreamOptions{SendToArmIntervalMs: 10, MaxRunwayInSessionMs: -1}).Validate(), test.ShouldNotBeNil)
}

// TestRunBackpressureWhenArmStalls checks that once the sampled backlog
// (max_runway_in_session_ms of PVATs) is full and the arm isn't consuming, the session
// stops accepting targets, so pushes block -- the backpressure signal a
// DoStreamPush caller relies on.
func TestRunBackpressureWhenArmStalls(t *testing.T) {
	inj := inject.NewArm("test-arm")
	inj.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context,
		batches <-chan []arm.TrajectoryPoint,
		responses chan<- arm.Response,
		extra map[string]interface{},
	) error {
		// A stalled arm: never consumes a batch, returns only on cancellation.
		<-ctx.Done()
		return ctx.Err()
	}

	// Small backlog (5 samples) and big, slow segments (~1s+ each at these
	// limits), so the gate closes after roughly one accepted target.
	opts := &StreamOptions{
		TargetRunwayInArmMs:  20,
		SendToArmIntervalMs:  10,
		VelLimitDegPerSec:    10,
		AccelLimitDegPerSec2: 10,
		MaxRunwayInSessionMs: 50,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jpCh := make(chan JointPositionsChItem)
	runErr := make(chan error, 1)
	go func() {
		runErr <- Run(ctx, inj, opts, jpCh, nil, []referenceframe.Input{0})
	}()

	send := func(timeout time.Duration, pos float64) error {
		sendCtx, sendCancel := context.WithTimeout(ctx, timeout)
		defer sendCancel()
		select {
		case jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{referenceframe.Input(pos)}}:
			return nil
		case <-sendCtx.Done():
			return sendCtx.Err()
		}
	}

	// The first target must be accepted: the session is idle.
	test.That(t, send(10*time.Second, 0.2), test.ShouldBeNil)

	// Keep pushing with a pause between attempts so sampling can fill the
	// backlog buffer (the bound is on *sampled* trajectory, so a same-instant
	// burst can slip in before the gate closes). With the arm stalled, a push
	// soon blocks past its timeout.
	blocked := false
	for i := 2; i <= 10; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := send(300*time.Millisecond, 0.2*float64(i)); err != nil {
			test.That(t, errors.Is(err, context.DeadlineExceeded), test.ShouldBeTrue)
			blocked = true
			break
		}
	}
	test.That(t, blocked, test.ShouldBeTrue)

	// Cancel tears the session down even though pushes were blocked.
	cancel()
	select {
	case err := <-runErr:
		test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
	case <-time.After(10 * time.Second):
		t.Fatal("session did not shut down after cancel")
	}
}
