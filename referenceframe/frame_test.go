package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
)

func TestStaticFrame(t *testing.T) {
	// define a static transform
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{1, 2, 3}, r3.Vector{0, 0, 1}, math.Pi/2)
	frame := &staticFrame{"test", expPose}
	// get expected transform back
	emptyInput := FloatsToInputs([]float64{})
	pose := frame.Transform(emptyInput)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in non-empty input, should get nil back
	nonEmptyInput := FloatsToInputs([]float64{0, 0, 0})
	pose = frame.Transform(nonEmptyInput)
	test.That(t, pose, test.ShouldBeNil)
	// check that there are no limits on the static frame
	min, max := frame.Limits()
	test.That(t, min, test.ShouldResemble, []float64{})
	test.That(t, max, test.ShouldResemble, []float64{})
}

func TestPrismaticFrame(t *testing.T) {
	// define a prismatic transform
	min, max := []float64{0, -30, 0}, []float64{0, 30, 0}
	frame := &prismaticFrame{"test", []bool{false, true, false}, min, max} // can only move on y axis
	// expected output
	expPose := spatial.NewPoseFromPoint(r3.Vector{0, 20, 0})
	// get expected transform back
	input := FloatsToInputs([]float64{20})
	pose := frame.Transform(input)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get nil back
	input = FloatsToInputs([]float64{0, 20, 0})
	pose = frame.Transform(input)
	test.That(t, pose, test.ShouldBeNil)
	// if you feed in empty input, should get nil back
	input = FloatsToInputs([]float64{})
	pose = frame.Transform(input)
	test.That(t, pose, test.ShouldBeNil)
	// gets the correct limits back
	reMin, reMax := frame.Limits()
	test.That(t, reMin, test.ShouldResemble, min)
	test.That(t, reMax, test.ShouldResemble, max)
	// change the form of the limits and see if it still works
	newMin, newMax := []float64{-30}, []float64{30}
	frame.SetLimits(newMin, newMax)
	reMin, reMax = frame.Limits()
	test.That(t, reMin, test.ShouldResemble, newMin)
	test.That(t, reMax, test.ShouldResemble, newMax)
}

func TestRevoluteFrame(t *testing.T) {
	// define a prismatic transform
	axis := spatial.R4AA{RX: 1, RY: 0, RZ: 0}                        // axis of rotation is x axis
	frame := &revoluteFrame{"test", axis, -math.Pi / 2, math.Pi / 2} // limits between -90 and 90 degrees
	// expected output
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, math.Pi/4) // 45 degrees
	// get expected transform back
	input := JointPosToInputs(&pb.JointPositions{Degrees: []float64{45}})
	pose := frame.Transform(input)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get nil back
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{45, 55}})
	pose = frame.Transform(input)
	test.That(t, pose, test.ShouldBeNil)
	// if you feed in empty input, should get nil back
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{}})
	pose = frame.Transform(input)
	test.That(t, pose, test.ShouldBeNil)
	// gets the correct limits back
	min, max := frame.Limits()
	test.That(t, min, test.ShouldResemble, []float64{-math.Pi / 2})
	test.That(t, max, test.ShouldResemble, []float64{math.Pi / 2})
}
