package kinematics

import (
	"testing"

	"go.viam.com/test"

	frame "go.viam.com/core/referenceframe"
)

func TestInterpolateValues(t *testing.T) {
	jp1 := frame.FloatsToInputs([]float64{0, 4})
	jp2 := frame.FloatsToInputs([]float64{8, -8})
	jpHalf := frame.FloatsToInputs([]float64{4, -2})
	jpQuarter := frame.FloatsToInputs([]float64{2, 1})

	interp1 := interpolateValues(jp1, jp2, 0.5)
	interp2 := interpolateValues(jp1, jp2, 0.25)
	test.That(t, interp1, test.ShouldResemble, jpHalf)
	test.That(t, interp2, test.ShouldResemble, jpQuarter)
}
