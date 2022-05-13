package boat

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	
	"go.viam.com/test"
)

const testTheta = .01

func TestWeights(t *testing.T) {

	type d struct {
		cfg motorConfig
		res motorWeights
	}

	tests := []d{
		{
			motorConfig{
				LateralOffset: 0,
				VerticalOffset: -10,
				AngleDegrees: 0,
				Weight: 1,
			},
			motorWeights{0, 1, 0},
		},
		{
			motorConfig{
				LateralOffset: 0,
				VerticalOffset: -10,
				AngleDegrees: 180,
				Weight: 1,
			},
			motorWeights{0, -1, 0},
		},
		{
			motorConfig{
				AngleDegrees: 45,
				Weight: math.Sqrt(2),
			},
			motorWeights{1, 1, 0},
		},
		{
			motorConfig{
				LateralOffset: -10,
				VerticalOffset: -10,
				AngleDegrees: 45,
				Weight: math.Sqrt(2),
			},
			motorWeights{1, 1, 0},
		},

		{
			motorConfig{
				AngleDegrees: 1, // this should be almost entirely linearY
				Weight: 1,
			},
			motorWeights{.017, .99, 0 },
		},
		{
			motorConfig{
				LateralOffset: 0,
				VerticalOffset: -10,
				AngleDegrees: 90,
				Weight: 1,
			},
			motorWeights{1, 0, -1},
		},
	}

	for _, x := range tests {
		t.Run(fmt.Sprintf("%#v", x), func(t *testing.T) {
			w := x.cfg.computeWeights(10)
			test.That(t, w.angular, test.ShouldAlmostEqual, x.res.angular, testTheta)
			test.That(t, w.linearX, test.ShouldAlmostEqual, x.res.linearX, testTheta)
			test.That(t, w.linearY, test.ShouldAlmostEqual, x.res.linearY, testTheta)
		})
	}

}

func TestBoatConfig(t *testing.T) {
	cfg := boatConfig{
		Motors: []motorConfig{
			{ Name : "starboard-rotation", LateralOffset: 0.3, VerticalOffset: 0, AngleDegrees: 0, Weight: 1},
			{ Name : "port-rotation", LateralOffset: -0.3, VerticalOffset: 0, AngleDegrees: 0, Weight: 1},
			{ Name : "forward", LateralOffset: 0, VerticalOffset: -0.3, AngleDegrees: 0, Weight: 1},
			{ Name : "reverse", LateralOffset: 0, VerticalOffset: 0.3, AngleDegrees: 180, Weight: 1},
			{ Name : "starboard-lateral", LateralOffset: 0.45, VerticalOffset: 0, AngleDegrees: 90, Weight: 1},
			{ Name : "port-lateral", LateralOffset: -0.45, VerticalOffset: 0, AngleDegrees: -90, Weight: 1},
		},
		Length: .5,
		Width: .5,
	}

	max := cfg.maxWeights()
	test.That(t, max.linearY, test.ShouldAlmostEqual, 4, testTheta)
	test.That(t, max.linearX, test.ShouldAlmostEqual, 2, testTheta)
	test.That(t, max.angular, test.ShouldAlmostEqual, .845, testTheta) // TODO(erh): is this right?

	g := cfg.computeGoal( r3.Vector{0,1,0,}, r3.Vector{})
	test.That(t, g.linearY, test.ShouldAlmostEqual, 4)

	powers := cfg.computePower(r3.Vector{0,1,0,}, r3.Vector{})
	test.That(t, powers[0], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	powers = cfg.computePower(r3.Vector{0,-1,0,}, r3.Vector{})
	test.That(t, powers[0], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	powers = cfg.computePower(r3.Vector{0,0,0,}, r3.Vector{Z: 1})
	test.That(t, powers[0], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	powers = cfg.computePower(r3.Vector{0,0,0,}, r3.Vector{Z: -1})
	test.That(t, powers[0], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

}
