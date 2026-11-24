package streaming

import (
	"context"
	"math"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
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
	s := NewArmStream(ctx, inj, diagnostics.New(0))

	// Empty PVAT list: nothing sent, wall clock not anchored.
	test.That(t, s.send(ctx, nil), test.ShouldBeNil)
	test.That(t, s.wallTimeAtTrajectoryClockZero.IsZero(), test.ShouldBeTrue)

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
	test.That(t, s.wallTimeAtTrajectoryClockZero.IsZero(), test.ShouldBeFalse)
	test.That(t, s.timeInTrajectoryClockOfLastSentPVAT, test.ShouldEqual, 10*time.Millisecond)

	// close waits for the RPC goroutine to finish, so the recorder is settled;
	// asserting on the batches before that would race the recorder's append.
	test.That(t, s.Close(), test.ShouldBeNil)
	batches := rec.get()
	test.That(t, len(batches), test.ShouldEqual, 1)
	test.That(t, len(batches[0]), test.ShouldEqual, 2)
	test.That(t, batches[0][0].Time, test.ShouldEqual, time.Duration(0))
	test.That(t, batches[0][1].Time, test.ShouldEqual, 10*time.Millisecond)
	// Velocities/accelerations pass through unconverted, in rad/s and rad/s^2.
	test.That(t, batches[0][0].Constraints.Velocities[0], test.ShouldAlmostEqual, math.Pi)
	test.That(t, batches[0][0].Constraints.Accelerations[1], test.ShouldAlmostEqual, math.Pi/2)
}

// TestArmStreamRunway checks that Runway returns 0
// before the first batch is sent, and afterward decreases as (real) time elapses.
func TestArmStreamRunway(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	ctx := context.Background()
	s := NewArmStream(ctx, inj, diagnostics.New(0))
	defer s.Close()

	test.That(t, s.Runway(), test.ShouldEqual, time.Duration(0))

	test.That(t, s.send(ctx, []pvat{testPVAT(100 * time.Millisecond)}), test.ShouldBeNil)

	prev := s.Runway()
	test.That(t, prev, test.ShouldBeLessThanOrEqualTo, 100*time.Millisecond)
	for prev > 0 {
		time.Sleep(5 * time.Millisecond)
		cur := s.Runway()
		test.That(t, cur, test.ShouldBeLessThanOrEqualTo, prev)
		prev = cur
	}
}

func TestArmStreamRunwayRestartsAfterArmRunsOut(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	ctx := context.Background()
	s := NewArmStream(ctx, inj, diagnostics.New(0))

	test.That(t, s.send(ctx, []pvat{testPVAT(0), testPVAT(10 * time.Millisecond)}), test.ShouldBeNil)

	s.wallTimeAtTrajectoryClockZero = time.Now().Add(-time.Second)
	test.That(t, s.Runway(), test.ShouldBeLessThan, -900*time.Millisecond)

	test.That(t, s.send(ctx, []pvat{testPVAT(60 * time.Millisecond)}), test.ShouldBeNil)
	runway := s.Runway()
	test.That(t, runway, test.ShouldBeGreaterThan, 30*time.Millisecond)
	test.That(t, runway, test.ShouldBeLessThanOrEqualTo, 50*time.Millisecond)

	test.That(t, s.Close(), test.ShouldBeNil)
}

func TestStopArmAfterCancel(t *testing.T) {
	inj, _ := newFakeStreamingArm()
	var stopCtxErr error
	stopped := false
	inj.StopFunc = func(ctx context.Context, _ map[string]any) error {
		stopped = true
		stopCtxErr = ctx.Err()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	test.That(t, StopArm(ctx, inj), test.ShouldBeNil)
	test.That(t, stopped, test.ShouldBeTrue)
	test.That(t, stopCtxErr, test.ShouldBeNil)
}
