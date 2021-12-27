package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/component/v1"
)

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Degrees[0], test.ShouldEqual, 0.0)
	test.That(t, j.Degrees[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

func TestBasicConversions(t *testing.T) {
	jp := &pb.ArmJointPositions{Degrees: []float64{45, 55}}
	inputs := JointPosToInputs(jp)
	test.That(t, jp, test.ShouldResemble, InputsToJointPos(inputs))

	floats := []float64{45, 55, 27}
	inputs = FloatsToInputs(floats)
	test.That(t, floats, test.ShouldResemble, InputsToFloats(inputs))
}

func TestInterpolateValues(t *testing.T) {
	jp1 := FloatsToInputs([]float64{0, 4})
	jp2 := FloatsToInputs([]float64{8, -8})
	jpHalf := FloatsToInputs([]float64{4, -2})
	jpQuarter := FloatsToInputs([]float64{2, 1})

	interp1 := InterpolateInputs(jp1, jp2, 0.5)
	interp2 := InterpolateInputs(jp1, jp2, 0.25)
	test.That(t, interp1, test.ShouldResemble, jpHalf)
	test.That(t, interp2, test.ShouldResemble, jpQuarter)
}
