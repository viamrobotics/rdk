package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Values[0], test.ShouldEqual, 0.0)
	test.That(t, j.Values[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

func TestInterpolateValues(t *testing.T) {
	jp1 := []Input{0, 4}
	jp2 := []Input{8, -8}
	jpHalf := []Input{4, -2}
	jpQuarter := []Input{2, 1}

	interp1 := interpolateInputs(jp1, jp2, 0.5)
	interp2 := interpolateInputs(jp1, jp2, 0.25)
	test.That(t, interp1, test.ShouldResemble, jpHalf)
	test.That(t, interp2, test.ShouldResemble, jpQuarter)
}
