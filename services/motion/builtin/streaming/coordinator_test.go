//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

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

func runTestOptions() StreamOptions {
	return StreamOptions{
		TargetRunwayInArmMs:  50,
		SendToArmIntervalMs:  10,
		VelLimitDegPerSec:    90,
		AccelLimitDegPerSec2: 90,
	}
}

func TestRunHappyPathStreamEndsViaJpChClose(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	jpCh := make(chan JointPositionsChItem)

	start := time.Now()
	trace := NewPipelineTrace()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), inj, runTestOptions(), jpCh, []referenceframe.Input{0, 0}, trace)
	}()

	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.05, -0.05}}
	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.1, -0.1}}
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

	// The trace carries one extend-disposition sample per pushed target, the first of
	// them the session's first build (which has no branch margin).
	var extendSamples []PipeSample
	for _, sample := range trace.Snapshot().Samples {
		if sample.Ch == pipeChanExtendBranch {
			extendSamples = append(extendSamples, sample)
		}
	}
	test.That(t, len(extendSamples), test.ShouldEqual, 2)
	test.That(t, extendSamples[0].Op, test.ShouldEqual, "first")
	test.That(t, extendSamples[0].Cap, test.ShouldEqual, 0)
}

// TestRunBackpressureGatesPushOnTrajexRunway covers MaxTrajexRunwayMs: a push made while
// the trajectory buffered inside trajex exceeds the cap is not accepted until execution
// drains the runway back under it.
func TestRunBackpressureGatesPushOnTrajexRunway(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	jpCh := make(chan JointPositionsChItem)
	opts := runTestOptions()
	// A 0.35 rad move at a 10 deg/s limit is roughly 2s of trajectory, far over the cap.
	opts.VelLimitDegPerSec = 10
	opts.MaxTrajexRunwayMs = 200

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), inj, opts, jpCh, []referenceframe.Input{0}, nil)
	}()

	// The first push is accepted immediately: the trajex runway is empty.
	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.35}}

	// The second push must stay blocked until the runway drains to under 200ms of the
	// ~2s trajectory, which takes execution (wall-clock) time.
	start := time.Now()
	select {
	case jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.36}}:
	case <-time.After(15 * time.Second):
		t.Fatal("gated push was never accepted")
	}
	test.That(t, time.Since(start), test.ShouldBeGreaterThan, 500*time.Millisecond)

	close(jpCh)
	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not finish after jpCh was closed")
	}
}

// TestRunBackpressureUnblocksOnCancel covers ending a session whose pusher is gated: Run
// must return on cancellation without ever accepting the gated push.
func TestRunBackpressureUnblocksOnCancel(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	jpCh := make(chan JointPositionsChItem)
	opts := runTestOptions()
	opts.VelLimitDegPerSec = 10
	opts.MaxTrajexRunwayMs = 200

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, inj, opts, jpCh, []referenceframe.Input{0}, nil)
	}()

	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.35}}

	// Leave a push pending against the closed gate, then cancel.
	pushAccepted := make(chan struct{})
	testDone := make(chan struct{})
	defer close(testDone)
	go func() {
		select {
		case jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.36}}:
			close(pushAccepted)
		case <-testDone:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return promptly after cancellation while a push was gated")
	}
	select {
	case <-pushAccepted:
		t.Fatal("the gated push was accepted even though the runway never drained")
	default:
	}
}

func TestRunEndsContextCanceled(t *testing.T) {
	t.Run("while streaming", func(t *testing.T) {
		inj, _ := newFakeStreamingArm()
		jpCh := make(chan JointPositionsChItem)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- Run(ctx, inj, runTestOptions(), jpCh, []referenceframe.Input{0}, nil)
		}()

		// The send on jpCh returning proves Run is in its loop; then cancel.
		jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.1}}
		cancel()

		select {
		case err := <-errCh:
			test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return promptly after cancellation mid-stream")
		}
	})

	t.Run("during post-flush wait", func(t *testing.T) {
		inj, _ := newFakeStreamingArm()
		jpCh := make(chan JointPositionsChItem, 1)
		// A 1.5 rad move is several seconds of trajectory, so the 100ms sleep below
		// lands well inside the post-flush wait.
		jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{1.5}}
		close(jpCh)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- Run(ctx, inj, runTestOptions(), jpCh, []referenceframe.Input{0}, nil)
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
	})
}

// TestRunEndsOnArmError covers the stream ending because the arm's streamed RPC fails:
// the arm's error surfaces in Run's returned error, without the caller closing jpCh.
func TestRunEndsOnArmError(t *testing.T) {
	armErr := errors.New("arm rejected the trajectory")
	inj := inject.NewArm("test-arm")
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

	jpCh := make(chan JointPositionsChItem)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), inj, runTestOptions(), jpCh, []referenceframe.Input{0}, nil)
	}()

	// One target is enough trajectory for several sends; the first is accepted, the
	// RPC dies, and the executor's next send discovers it.
	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.1}}

	select {
	case err := <-errCh:
		test.That(t, errors.Is(err, armErr), test.ShouldBeTrue)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the arm RPC failed")
	}
}
