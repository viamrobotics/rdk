//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testVelLimitRadPerSec    = math.Pi / 2 // 90 deg/s
	testAccelLimitRadPerSec2 = math.Pi / 2 // 90 deg/s^2
)

func runTestOptions() StreamOptions {
	runway, interval := int32(50), int32(10)
	return NewStreamOptions(motion.TempStreamOptions{
		ArmSideTargetRunwayMs: &runway,
		SendToArmIntervalMs:   &interval,
		MoveOptions:           &arm.MoveOptions{MaxVelRads: testVelLimitRadPerSec, MaxAccRads: testAccelLimitRadPerSec2},
	})
}

// ignoredAcks returns an acks channel whose acknowledgments are discarded in the background
// until the test ends, for calls whose acknowledgments are not under test.
func ignoredAcks(t *testing.T) chan motion.TempStreamResponse {
	t.Helper()
	acks := make(chan motion.TempStreamResponse)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range acks {
		}
	}()
	t.Cleanup(func() {
		close(acks)
		<-done
	})
	return acks
}

func TestRunRequiresAcks(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	jpCh := make(chan []referenceframe.Input)
	err := Run(context.Background(), inj, runTestOptions(), jpCh, []referenceframe.Input{0}, diagnostics.New(0), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "acks channel must be non-nil")
}

func TestRunHappyPathStreamEndsViaJpChClose(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	jpCh := make(chan []referenceframe.Input)

	start := time.Now()
	diag := diagnostics.New(time.Duration(runTestOptions().DiagnosticsWindowSecs) * time.Second)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), inj, runTestOptions(), jpCh, []referenceframe.Input{0, 0}, diag, ignoredAcks(t))
	}()

	jpCh <- []referenceframe.Input{0.05, -0.05}
	jpCh <- []referenceframe.Input{0.1, -0.1}
	close(jpCh)

	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not finish after jpCh was closed")
	}

	// A 0.1 rad move at 90 deg/s / 90 deg/s^2 limits (from runTestOptions()) is roughly
	// 500ms; assert that it was at least 250ms.
	test.That(t, time.Since(start), test.ShouldBeGreaterThan, 250*time.Millisecond)

	var lastPositions []referenceframe.Input
	var lastVelocities []float64
	prevTime := time.Duration(-1)
	points := 0
	for _, batch := range rec.get() {
		for _, p := range batch {
			test.That(t, len(p.Positions), test.ShouldEqual, 2)
			test.That(t, p.Time, test.ShouldBeGreaterThanOrEqualTo, prevTime)
			prevTime = p.Time
			lastPositions = p.Positions
			lastVelocities = p.Constraints.Velocities
			points++
		}
	}
	test.That(t, points, test.ShouldBeGreaterThan, 0)

	// The trajectory settles at the last pushed target, at rest.
	test.That(t, float64(lastPositions[0]), test.ShouldAlmostEqual, 0.1, 1e-2)
	test.That(t, float64(lastPositions[1]), test.ShouldAlmostEqual, -0.1, 1e-2)
	for _, v := range lastVelocities {
		test.That(t, v, test.ShouldAlmostEqual, 0, 0.05)
	}
	// A clean drain waits for the arm to finish on its own; nothing stops it.
	test.That(t, rec.stopCalls(), test.ShouldEqual, 0)

	snap := diag.LastWindowDetails()
	test.That(t, len(snap.JointPositionTargetReceived), test.ShouldEqual, 2)
	test.That(t, len(snap.ArmStreamOpen), test.ShouldEqual, 1)
	test.That(t, len(snap.TrajexSessionOpen), test.ShouldEqual, 1)
	test.That(t, len(snap.TrajexSessionClose), test.ShouldEqual, 1)
	test.That(t, len(snap.ArmStreamClose), test.ShouldEqual, 1)
	test.That(t, snap.ArmStreamOpen[0].TimestampMs, test.ShouldBeLessThanOrEqualTo, snap.TrajexSessionOpen[0].TimestampMs)
	test.That(t, snap.TrajexSessionOpen[0].TimestampMs, test.ShouldBeLessThanOrEqualTo, snap.TrajexSessionClose[0].TimestampMs)
	test.That(t, snap.TrajexSessionClose[0].TimestampMs, test.ShouldBeLessThanOrEqualTo, snap.ArmStreamClose[0].TimestampMs)
	test.That(t, len(snap.SampledPVATs), test.ShouldBeGreaterThan, 0)
	test.That(t, snap.ArmStreamOpen[0].TimestampMs, test.ShouldBeGreaterThan, 1e12)
}

// TestRunAcknowledgesEachTargetWithQueuedMs covers acks: each target is acknowledged once it is
// added to the trajectory, with the motion then queued inside trajex.
func TestRunAcknowledgesEachTargetWithQueuedMs(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	jpCh := make(chan []referenceframe.Input)
	acks := make(chan motion.TempStreamResponse)
	opts := runTestOptions()
	// A 0.35 rad move at a 10 deg/s limit is roughly 2s of trajectory.
	opts.MoveOptions.MaxVelRads = defaultVelLimitRadPerSec

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), inj, opts, jpCh, []referenceframe.Input{0}, diagnostics.New(0), acks)
	}()

	jpCh <- []referenceframe.Input{0.35}
	var first motion.TempStreamResponse
	select {
	case first = <-acks:
	case <-time.After(5 * time.Second):
		t.Fatal("the first target was never acknowledged")
	}
	test.That(t, first.QueuedMs, test.ShouldNotBeNil)
	test.That(t, *first.QueuedMs, test.ShouldBeGreaterThan, 1000)

	jpCh <- []referenceframe.Input{0.7}
	var second motion.TempStreamResponse
	select {
	case second = <-acks:
	case <-time.After(5 * time.Second):
		t.Fatal("the second target was never acknowledged")
	}
	test.That(t, second.QueuedMs, test.ShouldNotBeNil)
	// The second move adds more trajectory than execution drained in between.
	test.That(t, *second.QueuedMs, test.ShouldBeGreaterThan, *first.QueuedMs)

	close(jpCh)
	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not finish after jpCh was closed")
	}
}

func TestRunEndsContextCanceled(t *testing.T) {
	t.Run("while streaming", func(t *testing.T) {
		inj, rec := newFakeStreamingArm()
		jpCh := make(chan []referenceframe.Input)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- Run(ctx, inj, runTestOptions(), jpCh, []referenceframe.Input{0}, diagnostics.New(0), ignoredAcks(t))
		}()

		// The send on jpCh returning proves Run is in its loop; then cancel.
		jpCh <- []referenceframe.Input{0.1}
		cancel()

		select {
		case err := <-errCh:
			test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return promptly after cancellation mid-stream")
		}
		// An abort stops the arm explicitly, since the cancelled arm RPC does not wait for it.
		test.That(t, rec.stopCalls(), test.ShouldEqual, 1)
	})

	t.Run("during post-flush wait", func(t *testing.T) {
		inj, rec := newFakeStreamingArm()
		jpCh := make(chan []referenceframe.Input, 1)
		// A 1.5 rad move is several seconds of trajectory, so the 100ms sleep below
		// lands well inside the post-flush wait.
		jpCh <- []referenceframe.Input{1.5}
		close(jpCh)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- Run(ctx, inj, runTestOptions(), jpCh, []referenceframe.Input{0}, diagnostics.New(0), ignoredAcks(t))
		}()

		// Let the flush finish and the wait begin, then cancel.
		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case err := <-errCh:
			test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return promptly after cancellation during the post-flush wait")
		}
		test.That(t, rec.stopCalls(), test.ShouldEqual, 1)
	})
}

// TestRunEndsOnArmError covers the stream ending because the arm's streamed RPC fails:
// the arm's error surfaces in Run's returned error, without the caller closing jpCh.
func TestRunEndsOnArmError(t *testing.T) {
	armErr := errors.New("arm rejected the trajectory")
	inj := inject.NewArm("test-arm")
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
		// Accept one batch, then fail the RPC.
		<-batches
		return armErr
	}

	jpCh := make(chan []referenceframe.Input)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), inj, runTestOptions(), jpCh, []referenceframe.Input{0}, diagnostics.New(0), ignoredAcks(t))
	}()

	// One target is enough trajectory for several sends; the first is accepted, the
	// RPC dies, and the executor's next send discovers it.
	jpCh <- []referenceframe.Input{0.1}

	select {
	case err := <-errCh:
		test.That(t, errors.Is(err, armErr), test.ShouldBeTrue)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the arm RPC failed")
	}
}
