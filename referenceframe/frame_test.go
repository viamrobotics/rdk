package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"
)

func TestStaticFrame(t *testing.T) {
	// define a static transform
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{1, 2, 3}, r3.Vector{0, 0, 1}, math.Pi/2)
	frame := &staticFrame{"test", expPose}
	// get expected transform back
	emptyInput := FloatsToInputs([]float64{})
	pose, err := frame.Transform(emptyInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in non-empty input, should get err back
	nonEmptyInput := FloatsToInputs([]float64{0, 0, 0})
	_, err = frame.Transform(nonEmptyInput)
	test.That(t, err, test.ShouldNotBeNil)
	// check that there are no limits on the static frame
	limits := frame.Dof()
	test.That(t, limits, test.ShouldResemble, []Limit{})
}

func TestPrismaticFrame(t *testing.T) {
	// define a prismatic transform
	limits := []Limit{{-30, 30}}
	frame, err := NewPrismaticFrame("test", []bool{false, true, false}, limits) // can only move on y axis
	test.That(t, err, test.ShouldBeNil)
	// this should return an error
	badLimits := []Limit{{0, 0}, {-30, 30}, {0, 0}}
	_, err = NewPrismaticFrame("test", []bool{false, true, false}, badLimits) // can only move on y axis
	test.That(t, err, test.ShouldBeError, errors.New("given number of limits 3 does not match number of axes 1"))
	// expected output
	expPose := spatial.NewPoseFromPoint(r3.Vector{0, 20, 0})
	// get expected transform back
	input := FloatsToInputs([]float64{20})
	pose, err := frame.Transform(input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get an error back
	input = FloatsToInputs([]float64{0, 20, 0})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in empty input, should get an error
	input = FloatsToInputs([]float64{})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	overLimit := 50.0
	input = FloatsToInputs([]float64{overLimit})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f input out of bounds %.5f", overLimit, frame.Dof()[0]))
	// gets the correct limits back
	frameLimits := frame.Dof()
	test.That(t, frameLimits, test.ShouldResemble, limits)
}

func TestRevoluteFrame(t *testing.T) {
	// define a prismatic transform
	axis := spatial.R4AA{RX: 1, RY: 0, RZ: 0}                               // axis of rotation is x axis
	frame := &revoluteFrame{"test", axis, Limit{-math.Pi / 2, math.Pi / 2}} // limits between -90 and 90 degrees
	// expected output
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, math.Pi/4) // 45 degrees
	// get expected transform back
	input := JointPosToInputs(&pb.JointPositions{Degrees: []float64{45}})
	pose, err := frame.Transform(input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get error back
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{45, 55}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in empty input, should get errr back
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	overLimit := 100.0 // degrees
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{overLimit}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f input out of rev frame bounds %.5f", utils.DegToRad(overLimit), frame.Dof()[0]))
	// gets the correct limits back
	limit := frame.Dof()
	test.That(t, limit, test.ShouldResemble, []Limit{{-math.Pi / 2, math.Pi / 2}})
}
