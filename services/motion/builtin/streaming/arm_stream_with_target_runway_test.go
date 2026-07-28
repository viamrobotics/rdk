// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/testutils/inject"
)

// TestArmStreamAdd checks that addToBatch appends each PVAT as a trajectory point carrying its trajectory
// time, with velocities/accelerations passed through in radians unchanged (no unit conversion, and
// no time validation -- a bad trajectory is rejected downstream by the arm resource).
func TestArmStreamAdd(t *testing.T) {
	s := &armStreamWithTargetRunway{}

	s.addToBatch(pvat{
		positions:     []float64{0.1, 0.2},
		velocities:    []float64{math.Pi, 0},
		accelerations: []float64{0, math.Pi / 2},
		time:          0,
	})
	s.addToBatch(pvat{
		positions:     []float64{0.3, 0.4},
		velocities:    []float64{0, 0},
		accelerations: []float64{0, 0},
		time:          10 * time.Millisecond,
	})

	test.That(t, len(s.currentBatch), test.ShouldEqual, 2)
	test.That(t, s.currentBatch[0].Time, test.ShouldEqual, time.Duration(0))
	test.That(t, s.currentBatch[1].Time, test.ShouldEqual, 10*time.Millisecond)
	// Velocities/accelerations pass through unconverted, in rad/s and rad/s^2.
	test.That(t, s.currentBatch[0].Constraints.Velocities[0], test.ShouldAlmostEqual, math.Pi)
	test.That(t, s.currentBatch[0].Constraints.Accelerations[1], test.ShouldAlmostEqual, math.Pi/2)
}

// fakeStreamRecorder records the batches a fake arm receives over
// MoveThroughJointPositionsStreamed, guarded by a mutex since the batches are
// read from a separate goroutine than the one asserting on them.
type fakeStreamRecorder struct {
	mu      sync.Mutex
	batches [][]arm.TrajectoryPoint
}

func (r *fakeStreamRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.batches)
}

func newFakeStreamingArm() (*inject.Arm, *fakeStreamRecorder) {
	rec := &fakeStreamRecorder{}
	inj := inject.NewArm("test-arm")
	inj.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context,
		batches <-chan []arm.TrajectoryPoint,
		responses chan<- arm.Response,
		extra map[string]interface{},
	) error {
		for batch := range batches {
			rec.mu.Lock()
			rec.batches = append(rec.batches, batch)
			rec.mu.Unlock()
		}
		return nil
	}
	return inj, rec
}

func testPVAT(trajectoryTime time.Duration) pvat {
	return pvat{
		positions:     []float64{0},
		velocities:    []float64{0},
		accelerations: []float64{0},
		time:          trajectoryTime,
	}
}

// TestArmStreamSendHoldsUntilRunwayFilled checks that maybeSendBatch holds the first batch until
// pendingBatchDuration reaches targetRunway, and sends once it does.
func TestArmStreamSendHoldsUntilRunwayFilled(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	ctx := context.Background()
	s := &armStreamWithTargetRunway{arm: inj, targetRunway: 50 * time.Millisecond}
	s.startStream(ctx)

	s.addToBatch(testPVAT(0))
	s.addToBatch(testPVAT(20 * time.Millisecond))
	test.That(t, s.maybeSendBatch(ctx), test.ShouldBeNil)
	test.That(t, rec.count(), test.ShouldEqual, 0)
	test.That(t, s.firstBatchSent(), test.ShouldBeFalse)

	s.addToBatch(testPVAT(60 * time.Millisecond))
	test.That(t, s.maybeSendBatch(ctx), test.ShouldBeNil)
	test.That(t, s.firstBatchSent(), test.ShouldBeTrue)

	// close waits for the RPC goroutine to finish, so the recorder is settled;
	// asserting the count before that would race the recorder's append.
	test.That(t, s.close(), test.ShouldBeNil)
	test.That(t, rec.count(), test.ShouldEqual, 1)
}

// TestArmStreamSendDropsUnderfilledTrajectory checks that a trajectory that never
// accumulates a full targetRunway is silently dropped rather than sent underfilled --
// even across the maybeSendBatch calls a ticker and a subsequent flush would make.
func TestArmStreamSendDropsUnderfilledTrajectory(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	ctx := context.Background()
	s := &armStreamWithTargetRunway{arm: inj, targetRunway: 100 * time.Millisecond}
	s.startStream(ctx)

	s.addToBatch(testPVAT(10 * time.Millisecond))
	test.That(t, s.maybeSendBatch(ctx), test.ShouldBeNil) // a ticker tick
	test.That(t, s.maybeSendBatch(ctx), test.ShouldBeNil) // flush's drain
	test.That(t, s.close(), test.ShouldBeNil)

	test.That(t, rec.count(), test.ShouldEqual, 0)
	test.That(t, s.firstBatchSent(), test.ShouldBeFalse)
}

// TestArmStreamPendingBatchDuration checks that pendingBatchDuration reflects only the trajectory
// time queued in currentBatch since the last send -- 0 with an empty batch or before any send, and
// the span from the last sent PVAT to the newest batched one otherwise. This is the term run's
// accept-gate relies on to stop draining pvatCh once enough is already queued locally, since
// currentEstimatedRunwayInArm alone doesn't update until the next actual send.
func TestArmStreamPendingBatchDuration(t *testing.T) {
	s := &armStreamWithTargetRunway{}
	test.That(t, s.pendingBatchDuration(), test.ShouldEqual, time.Duration(0))

	s.addToBatch(testPVAT(30 * time.Millisecond))
	test.That(t, s.pendingBatchDuration(), test.ShouldEqual, 30*time.Millisecond)

	s.timeInTrajectoryClockOfLastSentPVAT = 20 * time.Millisecond
	test.That(t, s.pendingBatchDuration(), test.ShouldEqual, 10*time.Millisecond)
}

// TestArmStreamCurrentEstimatedRunwayInArm checks that currentEstimatedRunwayInArm returns 0
// before the first batch is sent, and afterward decreases as (real) time elapses.
func TestArmStreamCurrentEstimatedRunwayInArm(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	ctx := context.Background()
	s := &armStreamWithTargetRunway{arm: inj, targetRunway: 0}
	s.startStream(ctx)
	defer s.close()

	test.That(t, s.currentEstimatedRunwayInArm(), test.ShouldEqual, time.Duration(0))

	s.addToBatch(testPVAT(100 * time.Millisecond))
	test.That(t, s.maybeSendBatch(ctx), test.ShouldBeNil)

	runwayJustAfterSend := s.currentEstimatedRunwayInArm()
	test.That(t, runwayJustAfterSend, test.ShouldBeGreaterThan, 80*time.Millisecond)
	test.That(t, runwayJustAfterSend, test.ShouldBeLessThanOrEqualTo, 100*time.Millisecond)

	time.Sleep(30 * time.Millisecond)
	runwayAfterSleep := s.currentEstimatedRunwayInArm()
	test.That(t, runwayAfterSleep, test.ShouldBeLessThan, runwayJustAfterSend)
	test.That(t, runwayAfterSleep, test.ShouldBeGreaterThan, 40*time.Millisecond)
}
