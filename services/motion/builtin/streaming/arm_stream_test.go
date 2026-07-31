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

// fakeStreamRecorder records the batches a fake arm receives over
// MoveThroughJointPositionsStreamed, guarded by a mutex since the batches are
// read from a separate goroutine than the one asserting on them.
type fakeStreamRecorder struct {
	mu      sync.Mutex
	batches [][]arm.TrajectoryPoint
}

func (r *fakeStreamRecorder) get() [][]arm.TrajectoryPoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.batches
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

// TestArmStreamSend checks that send is a no-op on an empty PVAT list, and otherwise
// sends the PVATs to the arm as one batch of trajectory points, each carrying its PVAT's
// trajectory time with velocities/accelerations passed through in radians unchanged (no unit
// conversion, and no time validation -- a bad trajectory is rejected downstream by the arm
// resource). It also records the trajectory time of the last sent PVAT and anchors the runway
// estimate's wall clock on the first send.
func TestArmStreamSend(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	ctx := context.Background()
	s := &armStream{arm: inj}
	s.startStream(ctx)

	// Empty PVAT list: nothing sent, wall clock not anchored.
	test.That(t, s.send(ctx, nil), test.ShouldBeNil)
	test.That(t, s.timeFirstBatchWasSent.IsZero(), test.ShouldBeTrue)

	pvats := []pvat{
		{
			positions:     []float64{0.1, 0.2},
			velocities:    []float64{math.Pi, 0},
			accelerations: []float64{0, math.Pi / 2},
			time:          0,
		},
		{
			positions:     []float64{0.3, 0.4},
			velocities:    []float64{0, 0},
			accelerations: []float64{0, 0},
			time:          10 * time.Millisecond,
		},
	}
	test.That(t, s.send(ctx, pvats), test.ShouldBeNil)
	test.That(t, s.timeFirstBatchWasSent.IsZero(), test.ShouldBeFalse)
	test.That(t, s.timeInTrajectoryClockOfLastSentPVAT, test.ShouldEqual, 10*time.Millisecond)

	// close waits for the RPC goroutine to finish, so the recorder is settled;
	// asserting on the batches before that would race the recorder's append.
	test.That(t, s.close(), test.ShouldBeNil)
	batches := rec.get()
	test.That(t, len(batches), test.ShouldEqual, 1)
	test.That(t, len(batches[0]), test.ShouldEqual, 2)
	test.That(t, batches[0][0].Time, test.ShouldEqual, time.Duration(0))
	test.That(t, batches[0][1].Time, test.ShouldEqual, 10*time.Millisecond)
	// Velocities/accelerations pass through unconverted, in rad/s and rad/s^2.
	test.That(t, batches[0][0].Constraints.Velocities[0], test.ShouldAlmostEqual, math.Pi)
	test.That(t, batches[0][0].Constraints.Accelerations[1], test.ShouldAlmostEqual, math.Pi/2)
}

// TestArmStreamCurrentEstimatedRunwayInArm checks that currentEstimatedRunwayInArm returns 0
// before the first batch is sent, and afterward decreases as (real) time elapses.
func TestArmStreamCurrentEstimatedRunwayInArm(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	ctx := context.Background()
	s := &armStream{arm: inj}
	s.startStream(ctx)
	defer s.close()

	test.That(t, s.currentEstimatedRunwayInArm(), test.ShouldEqual, time.Duration(0))

	test.That(t, s.send(ctx, []pvat{testPVAT(100 * time.Millisecond)}), test.ShouldBeNil)

	runwayJustAfterSend := s.currentEstimatedRunwayInArm()
	test.That(t, runwayJustAfterSend, test.ShouldBeGreaterThan, 80*time.Millisecond)
	test.That(t, runwayJustAfterSend, test.ShouldBeLessThanOrEqualTo, 100*time.Millisecond)

	time.Sleep(30 * time.Millisecond)
	runwayAfterSleep := s.currentEstimatedRunwayInArm()
	test.That(t, runwayAfterSleep, test.ShouldBeLessThan, runwayJustAfterSend)
	test.That(t, runwayAfterSleep, test.ShouldBeGreaterThan, 40*time.Millisecond)
}
