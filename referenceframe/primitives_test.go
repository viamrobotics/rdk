package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/component/arm/v1"
)

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j[0].GetParameters(), test.ShouldResemble, []float64{0.0})
	test.That(t, j[0].GetJointType(), test.ShouldEqual, pb.JointPosition_JOINT_TYPE_REVOLUTE)
	test.That(t, j[1].GetParameters(), test.ShouldEqual, []float64{180.0})
	test.That(t, j[1].GetJointType(), test.ShouldEqual, pb.JointPosition_JOINT_TYPE_REVOLUTE)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

func TestBasicConversions(t *testing.T) {
	jp := []*pb.JointPosition{
		{
			Parameters: []float64{45},
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{55},
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	inputs, err := JointPosToInputs(jp)
	test.That(t, err, test.ShouldBeNil)
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

	interp1, err := InterpolateInputs(jp1, jp2, 0.5)
	test.That(t, err, test.ShouldBeNil)
	interp2, err := InterpolateInputs(jp1, jp2, 0.25)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interp1, test.ShouldResemble, jpHalf)
	test.That(t, interp2, test.ShouldResemble, jpQuarter)
}
