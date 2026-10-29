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
	biasAtCenter := NeutralBias(rotLimits, []bool{true, true, true}, []float64{0, 0, 0})
	test.That(t, biasAtCenter, test.ShouldEqual, 0)

	// At extremes, maximum bias
	biasAtPi := NeutralBias(rotLimits, []bool{true, true, true}, []float64{math.Pi, math.Pi, math.Pi})
	test.That(t, biasAtPi, test.ShouldBeGreaterThan, 0)

	// Symmetric extremes should produce equal bias
	biasAtNegPi := NeutralBias(rotLimits, []bool{true, true, true}, []float64{-math.Pi, -math.Pi, -math.Pi})
	test.That(t, biasAtNegPi, test.ShouldAlmostEqual, biasAtPi)

	// Closer to center should have less bias
	biasSmall := NeutralBias(rotLimits, []bool{true, true, true}, []float64{0.1, 0.1, 0.1})
	test.That(t, biasSmall, test.ShouldBeGreaterThan, 0)
	test.That(t, biasSmall, test.ShouldBeLessThan, biasAtPi)

	// Non-rotational limits should contribute no bias
	mixedLimits := []referenceframe.Limit{
		{Min: -math.Pi, Max: math.Pi}, // rotational
		{Min: 0, Max: 100},            // linear
	}
	biasNonRot := NeutralBias(mixedLimits, []bool{true, false}, []float64{math.Pi, 50})
	biasRotOnly := NeutralBias(mixedLimits, []bool{true, false}, []float64{math.Pi, 0})
	test.That(t, biasNonRot, test.ShouldEqual, biasRotOnly)

	// Asymmetric rotational limits: center is pi/2
	asymLimits := []referenceframe.Limit{
		{Min: 0, Max: math.Pi},
	}
	biasAtMid := NeutralBias(asymLimits, []bool{true}, []float64{math.Pi / 2})
	biasAtMin := NeutralBias(asymLimits, []bool{true}, []float64{0})
	biasAtMax := NeutralBias(asymLimits, []bool{true}, []float64{math.Pi})
	test.That(t, biasAtMid, test.ShouldEqual, 0)
	test.That(t, biasAtMin, test.ShouldAlmostEqual, biasAtMax)
	test.That(t, biasAtMin, test.ShouldBeGreaterThan, 0)

	// xarm-style limits: center is ~0
	xarmLimits := []referenceframe.Limit{
		{Min: -6.265732014659642, Max: 6.26573201465964},
	}
	xarmMid := (-6.265732014659642 + 6.26573201465964) / 2
	biasXarmCenter := NeutralBias(xarmLimits, []bool{true}, []float64{xarmMid})
	biasXarmPi := NeutralBias(xarmLimits, []bool{true}, []float64{math.Pi})
	test.That(t, biasXarmCenter, test.ShouldAlmostEqual, 0)
	test.That(t, biasXarmPi, test.ShouldBeGreaterThan, biasXarmCenter)

	// ±352°: revolute, though not multiples of π, so only the revolute flag can identify it.
	wideLimits := []referenceframe.Limit{{Min: -6.1435589670200397, Max: 6.1435589670200397}}
	test.That(t, wideLimits[0].IsRotational(), test.ShouldBeFalse)
	test.That(t, NeutralBias(wideLimits, []bool{true}, []float64{math.Pi}), test.ShouldBeGreaterThan, 0)
	test.That(t, NeutralBias(wideLimits, []bool{false}, []float64{math.Pi}), test.ShouldEqual, 0)
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
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot, rot}, []bool{true, true})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("exactly 2pi away is not unwrapped", func(t *testing.T) {
		cfg := []float64{twoPi}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("positive over-rotation unwraps down", func(t *testing.T) {
		// cfg moved by 2pi + 0.5 from start; unwrap subtracts one full rotation.
		cfg := []float64{twoPi + 0.5}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.5)
	})

	t.Run("negative over-rotation unwraps up", func(t *testing.T) {
		cfg := []float64{-twoPi - 0.5}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, -0.5)
	})

	t.Run("multiple full rotations collapse to within pi of start", func(t *testing.T) {
		// 4π + 0.3 away: rounds to 2 full rotations, lands at 0.3.
		cfg := []float64{4*math.Pi + 0.3}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.3)
		test.That(t, math.Abs(out[0]-start[0]), test.ShouldBeLessThan, math.Pi)
	})

	t.Run("non-rotational joints are skipped", func(t *testing.T) {
		// j0 is linear; even though it moved >2π, we leave it alone. j1 stayed
		// under 2π so nothing happens overall.
		cfg := []float64{100, 0.1}
		start := []float64{0, 0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{lin, rot}, []bool{false, true})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("unwrap candidate inside narrow rotational range", func(t *testing.T) {
		// Rotational joint with limit [-π, π]. cfg = 2π+0.5 unwraps to 0.5, in range.
		narrow := referenceframe.Limit{Min: -math.Pi, Max: math.Pi}
		test.That(t, narrow.IsRotational(), test.ShouldBeTrue)
		cfg := []float64{twoPi + 0.5}
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{narrow}, []bool{true})
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
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{narrow}, []bool{true})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out[0], test.ShouldEqual, cfg[0])
	})

	t.Run("multiple joints unwrap independently", func(t *testing.T) {
		cfg := []float64{twoPi + 0.2, 0.05, -twoPi - 0.3, 100}
		start := []float64{0, 0, 0, 0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot, rot, rot, lin}, []bool{true, true, true, false})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.2)  // unwrapped
		test.That(t, out[1], test.ShouldAlmostEqual, 0.05) // under 2π, untouched
		test.That(t, out[2], test.ShouldAlmostEqual, -0.3) // unwrapped
		test.That(t, out[3], test.ShouldAlmostEqual, 100)  // non-rotational, untouched
	})

	t.Run("revolute joint with limits that are not multiples of pi unwraps", func(t *testing.T) {
		wide := referenceframe.Limit{Min: -6.1435589670200397, Max: 6.1435589670200397} // ±352°
		test.That(t, wide.IsRotational(), test.ShouldBeFalse)
		start := []float64{-3}
		cfg := []float64{start[0] + twoPi + 0.5}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{wide}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, start[0]+0.5)
	})

	t.Run("input cfg is not mutated", func(t *testing.T) {
		cfg := []float64{twoPi + 0.2}
		orig := append([]float64{}, cfg...)
		start := []float64{0}
		out, changed := unwrapRotationalJoints(cfg, start, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, cfg, test.ShouldResemble, orig)
		test.That(t, out[0], test.ShouldNotEqual, cfg[0])
	})
}

func TestWrapTowardNeutral(t *testing.T) {
	const twoPi = 2 * math.Pi
	rot := referenceframe.Limit{Min: -twoPi, Max: twoPi}
	lin := referenceframe.Limit{Min: 0, Max: 10}

	t.Run("joint past half a turn from mid wraps to the nearer equivalent", func(t *testing.T) {
		// The wrist value from a captured plan: 3.947 rad has the in-range twin 3.947-2π ≈ -2.336.
		out, changed := wrapTowardNeutral([]float64{3.947}, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 3.947-twoPi)
	})

	t.Run("joint within half a turn of mid is left alone", func(t *testing.T) {
		cfg := []float64{3.0, -3.0}
		out, changed := wrapTowardNeutral(cfg, []referenceframe.Limit{rot, rot}, []bool{true, true})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out, test.ShouldResemble, cfg)
	})

	t.Run("wraps toward an off-center midpoint", func(t *testing.T) {
		// Midpoint 2π; 0.1 is 6.2 rad below it, 0.1+2π just 0.1 rad above.
		offCenter := referenceframe.Limit{Min: -math.Pi, Max: 5 * math.Pi}
		out, changed := wrapTowardNeutral([]float64{0.1}, []referenceframe.Limit{offCenter}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 0.1+twoPi)
	})

	t.Run("limits need not be multiples of pi", func(t *testing.T) {
		// ±352°, as on a real arm whose joints Limit.IsRotational does not recognize.
		limit := referenceframe.Limit{Min: -6.1435589670200397, Max: 6.1435589670200397}
		out, changed := wrapTowardNeutral([]float64{3.947}, []referenceframe.Limit{limit}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldAlmostEqual, 3.947-twoPi)
	})

	t.Run("twin outside the limits is rejected", func(t *testing.T) {
		// Range under a full turn: no whole-turn equivalent fits.
		narrow := referenceframe.Limit{Min: -2, Max: 2}
		out, changed := wrapTowardNeutral([]float64{1.9}, []referenceframe.Limit{narrow}, []bool{true})
		test.That(t, changed, test.ShouldBeFalse)
		test.That(t, out[0], test.ShouldEqual, 1.9)
	})

	t.Run("joints not flagged wrappable are left alone", func(t *testing.T) {
		cfg := []float64{5.0, 9.0, 5.0}
		out, changed := wrapTowardNeutral(cfg, []referenceframe.Limit{rot, lin, rot}, []bool{false, false, true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, out[0], test.ShouldEqual, 5.0)
		test.That(t, out[1], test.ShouldEqual, 9.0)
		test.That(t, out[2], test.ShouldAlmostEqual, 5.0-twoPi)
	})

	t.Run("input cfg is not mutated", func(t *testing.T) {
		cfg := []float64{5.0}
		out, changed := wrapTowardNeutral(cfg, []referenceframe.Limit{rot}, []bool{true})
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, cfg[0], test.ShouldEqual, 5.0)
		test.That(t, out[0], test.ShouldNotEqual, cfg[0])
	})
}

func TestMostNeutralGoals(t *testing.T) {
	rot := referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi}
	limits := []referenceframe.Limit{rot, rot}
	goal := func(cost float64, cfg ...float64) *node {
		return &node{inputs: referenceframe.FrameSystemInputs{"a": cfg}.ToLinearInputs(), cost: cost}
	}

	// Cost-sorted, as getSolutions returns them: the nearest-to-start goal is the least neutral.
	near := goal(1, 3.9, 0)
	wrapped := goal(2, 3.9-2*math.Pi, 0)
	alsoNeutral := goal(3, 3.9-2*math.Pi, 0.3)
	far := goal(4, 3.9, 0.3)

	tier := mostNeutralGoals(limits, []*node{near, wrapped, alsoNeutral, far})
	test.That(t, tier, test.ShouldResemble, []*node{wrapped, alsoNeutral})

	tier = mostNeutralGoals(limits, []*node{wrapped})
	test.That(t, tier, test.ShouldResemble, []*node{wrapped})
}
