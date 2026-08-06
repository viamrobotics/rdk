//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

func runTestOptions() *StreamOptions {
	return &StreamOptions{
		TargetRunwayInArmMs:  50,
		SendToArmIntervalMs:  10,
		VelLimitDegPerSec:    90,
		AccelLimitDegPerSec2: 90,
	}
}

// TestRunFlushWaitsOutRunway checks that after jpCh closes and the remaining trajectory
// has been sent, Run does not return until the arm has (by the runway estimate) finished
// executing it. The fake arm's streamed RPC returns as soon as the batch channel closes,
// so without the wait Run would return near-instantly with most of the trajectory still
// "executing".
func TestRunFlushWaitsOutRunway(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	jpCh := make(chan JointPositionsChItem, 1)
	// One 0.1 rad move at 90 deg/s / 90 deg/s^2 limits: a triangular profile of roughly
	// half a second of trajectory.
	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.1}}
	close(jpCh)

	start := time.Now()
	err := Run(context.Background(), inj, runTestOptions(), jpCh, []referenceframe.Input{0}, nil)
	elapsed := time.Since(start)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, elapsed, test.ShouldBeGreaterThan, 250*time.Millisecond)
	test.That(t, elapsed, test.ShouldBeLessThan, 5*time.Second)
	test.That(t, len(rec.get()), test.ShouldBeGreaterThan, 0)
}

// TestRunFlushWaitCancellable checks that cancellation cuts the post-drain wait short
// rather than holding an abort hostage to the estimate.
func TestRunFlushWaitCancellable(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	jpCh := make(chan JointPositionsChItem, 1)
	jpCh <- JointPositionsChItem{Positions: []referenceframe.Input{0.1}}
	close(jpCh)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, inj, runTestOptions(), jpCh, []referenceframe.Input{0}, nil)
	}()

	// Let the drain finish and the wait begin, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return promptly after cancellation during the post-drain wait")
	}
}
