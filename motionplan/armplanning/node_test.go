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
			&basicNode{
				q: referenceframe.FrameSystemInputs{
					"a": {{x[0]}},
				},
			},
			&basicNode{
				q: referenceframe.FrameSystemInputs{
					"a": {{x[1]}},
				},
			},
			map[string][]float64{"a": {x[2]}},
		)
		test.That(t, res["a"][0].Value, test.ShouldEqual, x[3])
	}
}
