package tpspace

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func TestPtgNegativePartialTransform(t *testing.T) {
	// diff drive only ptg group frame
	pFrame, err := NewPTGFrameFromKinematicOptions(
		"",
		logging.NewTestLogger(t),
		1.,
		2,
		nil,
		true,
		true,
	)
	test.That(t, err, test.ShouldBeNil)
	pf, ok := pFrame.(*ptgGroupFrame)
	test.That(t, ok, test.ShouldBeTrue)

	partialPose1, err := pf.Transform([]referenceframe.Input{{0}, {math.Pi / 2}, {200}, {30}})
	test.That(t, err, test.ShouldBeNil)
	partialPose2, err := pf.Transform([]referenceframe.Input{{0}, {math.Pi / 2}, {200}, {40}})
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		spatialmath.PoseAlmostEqual(
			partialPose1,
			spatialmath.Compose(partialPose2, spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -10})),
		),
		test.ShouldBeTrue,
	)
}

func TestInterpolate(t *testing.T) {
	t.Parallel()
	// diff drive only ptg group frame
	pFrame, err := NewPTGFrameFromKinematicOptions(
		"",
		logging.NewTestLogger(t),
		1.,
		2,
		nil,
		false,
		true,
	)
	test.That(t, err, test.ShouldBeNil)

	zeroInput := make([]referenceframe.Input, 4)

	type testCase struct {
		name      string
		inputFrom []referenceframe.Input
		inputTo   []referenceframe.Input
		expected  []referenceframe.Input
		amount    float64
	}

	testCases := []testCase{
		{
			name:      "Simple interpolation 1",
			inputFrom: zeroInput,
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {0}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {0}, {80}},
			amount:    0.4,
		},
		{
			name:      "Simple interpolation 2",
			inputFrom: zeroInput,
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {0}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {0}, {120}},
			amount:    0.6,
		},
		{
			name:      "Simple interpolation 3",
			inputFrom: []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {100}},
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {140}},
			amount:    0.4,
		},
		{
			name:      "Simple interpolation 4",
			inputFrom: zeroInput,
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {140}},
			amount:    0.4,
		},
		{
			name:      "Nonzero starting point",
			inputFrom: []referenceframe.Input{{0}, {math.Pi / 2}, {0}, {100}},
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {0}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {140}},
			amount:    0.4,
		},
		{
			name:      "Zero interpolation",
			inputFrom: []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {100}},
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {100}},
			amount:    0,
		},
		{
			name:      "Zero interpolation with zero input",
			inputFrom: zeroInput,
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {200}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {100}},
			amount:    0,
		},
		{
			name:      "Reverse interpolation 1",
			inputFrom: zeroInput,
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {0}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {120}},
			amount:    0.4,
		},
		{
			name:      "Reverse interpolation 2",
			inputFrom: []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {200}},
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {0}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {120}},
			amount:    0.4,
		},
		{
			name:      "Reverse interpolation nonzero starting point",
			inputFrom: []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {100}},
			inputTo:   []referenceframe.Input{{0}, {math.Pi / 2}, {200}, {0}},
			expected:  []referenceframe.Input{{0}, {math.Pi / 2}, {100}, {60}},
			amount:    0.4,
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		interp, err := pFrame.Interpolate(tc.inputFrom, tc.inputTo, tc.amount)
		if tc.expected != nil {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, interp, test.ShouldResemble, tc.expected)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
		}
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testFn(t, c)
		})
	}
}
