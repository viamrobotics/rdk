package boat

import (
	"fmt"
	"math"
	"testing"

	"go.viam.com/test"
)

const testTheta = .01

func TestMotorWeights(t *testing.T) {
	type d struct {
		cfg motorConfig
		res motorWeights
	}

	tests := []d{
		{
			motorConfig{
				XOffset:      0,
				YOffset:      -10,
				AngleDegrees: 0,
				Weight:       1,
			},
			motorWeights{0, 1, 0},
		},
		{
			motorConfig{
				XOffset:      0,
				YOffset:      -10,
				AngleDegrees: 180,
				Weight:       1,
			},
			motorWeights{0, -1, 0},
		},
		{
			motorConfig{
				AngleDegrees: 45,
				Weight:       math.Sqrt(2),
			},
			motorWeights{1, 1, 0},
		},
		{
			motorConfig{
				XOffset:      -10,
				YOffset:      -10,
				AngleDegrees: 45,
				Weight:       math.Sqrt(2),
			},
			motorWeights{1, 1, 0},
		},

		{
			motorConfig{
				AngleDegrees: 1, // this should be almost entirely linearY
				Weight:       1,
			},
			motorWeights{.017, .99, 0},
		},
		{
			motorConfig{
				XOffset:      0,
				YOffset:      -10,
				AngleDegrees: 90,
				Weight:       1,
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
