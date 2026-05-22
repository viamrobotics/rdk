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
	// Symmetric limits: center is 0
	rotLimits := []referenceframe.Limit{
		{Min: -math.Pi, Max: math.Pi},
		{Min: -math.Pi, Max: math.Pi},
		{Min: -math.Pi, Max: math.Pi},
	}

	// At center (0), no bias
	biasAtCenter := neutralBias(rotLimits, []float64{0, 0, 0})
	test.That(t, biasAtCenter, test.ShouldEqual, 0)

	// At extremes, maximum bias
	biasAtPi := neutralBias(rotLimits, []float64{math.Pi, math.Pi, math.Pi})
	test.That(t, biasAtPi, test.ShouldBeGreaterThan, 0)

	// Symmetric extremes should produce equal bias
	biasAtNegPi := neutralBias(rotLimits, []float64{-math.Pi, -math.Pi, -math.Pi})
	test.That(t, biasAtNegPi, test.ShouldAlmostEqual, biasAtPi)

	// Closer to center should have less bias
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

	// Asymmetric rotational limits: center is pi/2
	asymLimits := []referenceframe.Limit{
		{Min: 0, Max: math.Pi},
	}
	biasAtMid := neutralBias(asymLimits, []float64{math.Pi / 2})
	biasAtMin := neutralBias(asymLimits, []float64{0})
	biasAtMax := neutralBias(asymLimits, []float64{math.Pi})
	test.That(t, biasAtMid, test.ShouldEqual, 0)
	test.That(t, biasAtMin, test.ShouldAlmostEqual, biasAtMax)
	test.That(t, biasAtMin, test.ShouldBeGreaterThan, 0)

	// xarm-style limits: center is ~0
	xarmLimits := []referenceframe.Limit{
		{Min: -6.265732014659642, Max: 6.26573201465964},
	}
	xarmMid := (-6.265732014659642 + 6.26573201465964) / 2
	biasXarmCenter := neutralBias(xarmLimits, []float64{xarmMid})
	biasXarmPi := neutralBias(xarmLimits, []float64{math.Pi})
	test.That(t, biasXarmCenter, test.ShouldAlmostEqual, 0)
	test.That(t, biasXarmPi, test.ShouldBeGreaterThan, biasXarmCenter)
}

func TestUnwrapRotationalJoints(t *testing.T) {
	const twoPi = 2 * math.Pi
	rot := referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi} // rotational
	lin := referenceframe.Limit{Min: 0, Max: 1}                      // not rotational
	test.That(t, rot.IsRotational(), test.ShouldBeTrue)
	test.That(t, lin.IsRotational(), test.ShouldBeFalse)

	t.Run("no joints moved beyond 2pi", func(t *testing.T) {
		cfg := []float64{0.5, -0.5}
		start := []float64{0, 0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot, rot})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("exactly 2pi away is not unwrapped", func(t *testing.T) {
		cfg := []float64{twoPi}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("positive over-rotation unwraps down", func(t *testing.T) {
		// cfg moved by 2pi + 0.5 from start; unwrap subtracts one full rotation.
		cfg := []float64{twoPi + 0.5}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.5)
	})

	t.Run("negative over-rotation unwraps up", func(t *testing.T) {
		cfg := []float64{-twoPi - 0.5}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, -0.5)
	})

	t.Run("multiple full rotations collapse to within pi of start", func(t *testing.T) {
		// 4π + 0.3 away: rounds to 2 full rotations, lands at 0.3.
		cfg := []float64{4*math.Pi + 0.3}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.3)
		test.That(t, math.Abs(out[0]-start[0]), test.ShouldBeLessThan, math.Pi)
	})

	t.Run("non-rotational joints are skipped", func(t *testing.T) {
		// j0 is linear; even though it moved >2π, we leave it alone. j1 stayed
		// under 2π so nothing happens overall.
		cfg := []float64{100, 0.1}
		start := []float64{0, 0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{lin, rot})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("unwrap candidate inside narrow rotational range", func(t *testing.T) {
		// Rotational joint with limit [-π, π]. cfg = 2π+0.5 unwraps to 0.5, in range.
		narrow := referenceframe.Limit{Min: -math.Pi, Max: math.Pi}
		test.That(t, narrow.IsRotational(), test.ShouldBeTrue)
		cfg := []float64{twoPi + 0.5}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{narrow})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.5)
	})

	t.Run("unwrap candidate outside narrow rotational range is rejected", func(t *testing.T) {
		// Rotational joint with limit [-π, π]; start sits near the upper edge so
		// the unwrap candidate (start + (diff - 2π)) lands above π and must be
		// left alone, even though the unwrap would reduce joint travel.
		narrow := referenceframe.Limit{Min: -math.Pi, Max: math.Pi}
		start := []float64{math.Pi - 0.1}
		cfg := []float64{start[0] + twoPi + 3.0} // unwrap candidate ≈ π+2.9 > π
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{narrow})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out[0], test.ShouldEqual, cfg[0])
	})

	t.Run("multiple joints unwrap independently", func(t *testing.T) {
		cfg := []float64{twoPi + 0.2, 0.05, -twoPi - 0.3, 100}
		start := []float64{0, 0, 0, 0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot, rot, rot, lin})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.2)  // unwrapped
		test.That(t, out[1], test.ShouldAlmostEqual, 0.05) // under 2π, untouched
		test.That(t, out[2], test.ShouldAlmostEqual, -0.3) // unwrapped
		test.That(t, out[3], test.ShouldAlmostEqual, 100)  // non-rotational, untouched
	})

	t.Run("input cfg is not mutated", func(t *testing.T) {
		cfg := []float64{twoPi + 0.2}
		orig := append([]float64{}, cfg...)
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, cfg, test.ShouldResemble, orig)
		test.That(t, out[0], test.ShouldNotEqual, cfg[0])
	})
}
