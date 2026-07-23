package builtin

import (
	"math"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestStreamConfigApplyDefaultsAndValidate(t *testing.T) {
	c := streamConfig{}
	c.ApplyDefaults()
	test.That(t, c.BufferAheadInArmMs, test.ShouldEqual, defaultBufferAheadInArmMs)
	test.That(t, c.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, c.VelLimitDegPerSec, test.ShouldEqual, defaultVelLimitDegPerSec)
	test.That(t, c.AccelLimitDegPerSec2, test.ShouldEqual, defaultAccelLimitDegPerSec2)
	test.That(t, c.Validate(), test.ShouldBeNil)

	test.That(t, (&streamConfig{SendToArmIntervalMs: 10, BufferAheadInArmMs: -1}).Validate(), test.ShouldNotBeNil)
	// A zero send interval is invalid (division by zero when converting to Hz).
	test.That(t, (&streamConfig{SendToArmIntervalMs: 0, BufferAheadInArmMs: 10}).Validate(), test.ShouldNotBeNil)
	test.That(t, (&streamConfig{SendToArmIntervalMs: 10, VelLimitDegPerSec: -1}).Validate(), test.ShouldNotBeNil)
}

// TestArmStreamAdd exercises the strictly-increasing-time contract and the
// rad->deg conversion in armStream.add without touching trajex or the arm.
func TestArmStreamAdd(t *testing.T) {
	s := &armStream{}

	// First point may have zero time.
	test.That(t, s.add(pvat{
		positions:     []float64{0.1, 0.2},
		velocities:    []float64{math.Pi, 0},
		accelerations: []float64{0, math.Pi / 2},
		time:          0,
	}), test.ShouldBeNil)

	// Strictly-increasing time is accepted.
	test.That(t, s.add(pvat{
		positions:     []float64{0.3, 0.4},
		velocities:    []float64{0, 0},
		accelerations: []float64{0, 0},
		time:          10 * time.Millisecond,
	}), test.ShouldBeNil)

	// Zero dt after the first point is rejected.
	test.That(t, s.add(pvat{
		positions: []float64{0.3, 0.4}, velocities: []float64{0, 0}, accelerations: []float64{0, 0},
		time: 10 * time.Millisecond,
	}), test.ShouldNotBeNil)

	// Negative dt is rejected.
	test.That(t, s.add(pvat{
		positions: []float64{0.3, 0.4}, velocities: []float64{0, 0}, accelerations: []float64{0, 0},
		time: 5 * time.Millisecond,
	}), test.ShouldNotBeNil)

	// Only the two accepted points were appended.
	test.That(t, len(s.points), test.ShouldEqual, 2)
	test.That(t, s.points[0].Time, test.ShouldEqual, time.Duration(0))
	test.That(t, s.points[1].Time, test.ShouldEqual, 10*time.Millisecond)
	// pi rad/s -> 180 deg/s.
	test.That(t, s.points[0].Constraints.Velocities[0], test.ShouldAlmostEqual, 180.0)
}
