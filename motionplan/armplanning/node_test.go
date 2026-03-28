package armplanning

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

func TestFixedStepInterpolation(t *testing.T) {
	xx := [][]float64{
		{5, 7, .5, 5.5},
		{5, 7, 0, 5},
		{5, 7, 3, 7},
		{5, 7, 1, 6},
		{7, 5, 1, 6},
	}
	for _, x := range xx {
		res := fixedStepInterpolation(
			&node{
				inputs: referenceframe.FrameSystemInputs{
					"a": {x[0]},
				}.ToLinearInputs(),
			},
			&node{
				inputs: referenceframe.FrameSystemInputs{
					"a": {x[1]},
				}.ToLinearInputs(),
			},
			map[string][]float64{"a": {x[2]}},
		)
		test.That(t, res.Get("a")[0], test.ShouldEqual, x[3])
	}
}

func TestNeutralBias(t *testing.T) {
	rotLimits := []referenceframe.Limit{
		{Min: -math.Pi, Max: math.Pi},
		{Min: -math.Pi, Max: math.Pi},
		{Min: -math.Pi, Max: math.Pi},
	}

	// At 0, no bias
	biasAtZero := neutralBias(rotLimits, []float64{0, 0, 0})
	test.That(t, biasAtZero, test.ShouldEqual, 0)

	// At pi, maximum bias
	biasAtPi := neutralBias(rotLimits, []float64{math.Pi, math.Pi, math.Pi})
	test.That(t, biasAtPi, test.ShouldBeGreaterThan, 0)

	// At -pi, same as pi
	biasAtNegPi := neutralBias(rotLimits, []float64{-math.Pi, -math.Pi, -math.Pi})
	test.That(t, biasAtNegPi, test.ShouldAlmostEqual, biasAtPi)

	// Closer to 0 should have less bias
	biasSmall := neutralBias(rotLimits, []float64{0.1, 0.1, 0.1})
	test.That(t, biasSmall, test.ShouldBeGreaterThan, 0)
	test.That(t, biasSmall, test.ShouldBeLessThan, biasAtPi)

	// Non-rotational limits should contribute no bias
	mixedLimits := []referenceframe.Limit{
		{Min: -math.Pi, Max: math.Pi}, // rotational
		{Min: 0, Max: 100},            // linear
	}
	biasNonRot := neutralBias(mixedLimits, []float64{math.Pi, 50})
	biasRotOnly := neutralBias(mixedLimits, []float64{math.Pi, 0})
	test.That(t, biasNonRot, test.ShouldEqual, biasRotOnly)

	// xarm-style limits (-2pi to 2pi) should also count as rotational
	xarmLimits := []referenceframe.Limit{
		{Min: -6.265732014659642, Max: 6.26573201465964},
	}
	biasXarmZero := neutralBias(xarmLimits, []float64{0})
	biasXarmPi := neutralBias(xarmLimits, []float64{math.Pi})
	test.That(t, biasXarmZero, test.ShouldEqual, 0)
	test.That(t, biasXarmPi, test.ShouldBeGreaterThan, 0)
}
