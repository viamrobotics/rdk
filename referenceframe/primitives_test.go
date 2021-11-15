package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/v1"
)

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Degrees[0], test.ShouldEqual, 0.0)
	test.That(t, j.Degrees[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

func TestBasicConversions(t *testing.T) {
	jp := &pb.JointPositions{Degrees: []float64{45, 55}}
	inputs := JointPosToInputs(jp)
	test.That(t, jp, test.ShouldResemble, InputsToJointPos(inputs))

	floats := []float64{45, 55, 27}
	inputs = FloatsToInputs(floats)
	test.That(t, floats, test.ShouldResemble, InputsToFloats(inputs))
}
