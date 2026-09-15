package streaming

import (
	"context"
	"math"
	"testing"
	"time"

	"go.viam.com/test"
)

func testPVAT(trajectoryTime time.Duration) pvat {
	return pvat{
		positions:     []float64{0},
		velocities:    []float64{0},
		accelerations: []float64{0},
		time:          trajectoryTime,
	}
}

func TestArmStreamSend(t *testing.T) {
	inj, rec := newFakeStreamingArm()
	ctx := context.Background()
	s := newArmStream(ctx, inj, nil)

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
	s := newArmStream(ctx, inj, nil)
	defer s.close()

	test.That(t, s.currentEstimatedRunwayInArm(), test.ShouldEqual, time.Duration(0))

	test.That(t, s.send(ctx, []pvat{testPVAT(100 * time.Millisecond)}), test.ShouldBeNil)

	prev := s.currentEstimatedRunwayInArm()
	test.That(t, prev, test.ShouldBeLessThanOrEqualTo, 100*time.Millisecond)
	for prev > 0 {
		time.Sleep(5 * time.Millisecond)
		cur := s.currentEstimatedRunwayInArm()
		test.That(t, cur, test.ShouldBeLessThanOrEqualTo, prev)
		prev = cur
	}
}
