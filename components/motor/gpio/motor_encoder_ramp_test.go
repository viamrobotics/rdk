package gpio

import (
	"testing"
)

func TestWrapMotorWithEncoderRampMath(t *testing.T) {
	t.Skip()
	// m := EncodedMotor{rampRate: 0.5, maxPowerPct: 1.0}

	// test.That(t, m.computeRamp(0, 1), test.ShouldEqual, .5)
	// test.That(t, m.computeRamp(0.5, 1), test.ShouldEqual, .75)

	// m.rampRate = 1
	// test.That(t, m.computeRamp(0, 1), test.ShouldEqual, 1)
	// test.That(t, m.computeRamp(0.5, 1), test.ShouldEqual, 1)
	// test.That(t, m.computeRamp(0.5, .9), test.ShouldEqual, .9)

	// m.rampRate = .25
	// test.That(t, m.computeRamp(0, 1), test.ShouldEqual, .25)
	// test.That(t, m.computeRamp(0.5, 1), test.ShouldEqual, .625)
	// test.That(t, m.computeRamp(0.999, 1), test.ShouldEqual, 1)

	// test.That(t, m.computeRamp(.8-(1.0/255.0), .8), test.ShouldEqual, .8)
	// test.That(t, m.computeRamp(.8+(1.0/255.0), .8), test.ShouldEqual, .8)
	// test.That(t, m.computeRamp(.8-(2.0/255.0), .8), test.ShouldAlmostEqual, .7941176, .0000001)
	// test.That(t, m.computeRamp(.8+(2.0/255.0), .8), test.ShouldAlmostEqual, .8058823, .0000001)

	// m = EncodedMotor{rampRate: 0.5, maxPowerPct: 0.65}
	// test.That(t, m.computeRamp(0.5, 1), test.ShouldEqual, .65)
	// test.That(t, m.computeRamp(0.65, 0.9), test.ShouldEqual, .65)
	// test.That(t, m.computeRamp(0.2, 1), test.ShouldAlmostEqual, .6, 0.001)
	// test.That(t, m.computeRamp(0.7, 0.701), test.ShouldAlmostEqual, .65, 0.001)
}
