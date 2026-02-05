package armplanning

import (
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
