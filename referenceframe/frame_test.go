package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/component/v1"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
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
	limits := frame.DoF()
	test.That(t, limits, test.ShouldResemble, []Limit{})

	errExpect := errors.New("pose is not allowed to be nil")
	f, err := NewStaticFrame("test2", nil)
	test.That(t, err.Error(), test.ShouldEqual, errExpect.Error())
	test.That(t, f, test.ShouldBeNil)
}

func TestPrismaticFrame(t *testing.T) {
	// define a prismatic transform
	limits := []Limit{{Min: -30, Max: 30}}
	frame, err := NewTranslationalFrame("test", []bool{false, true, false}, limits) // can only move on y axis
	test.That(t, err, test.ShouldBeNil)
	// this should return an error
	badLimits := []Limit{{Min: 0, Max: 0}, {Min: -30, Max: 30}, {Min: 0, Max: 0}}
	_, err = NewTranslationalFrame("test", []bool{false, true, false}, badLimits) // can only move on y axis
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
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f input out of bounds %v", overLimit, frame.DoF()[0]))
	// gets the correct limits back
	frameLimits := frame.DoF()
	test.That(t, frameLimits, test.ShouldResemble, limits)

	randomInputs := RandomFrameInputs(frame, nil)
	test.That(t, len(randomInputs), test.ShouldEqual, len(frame.DoF()))
	restrictRandomInputs := RestrictedRandomFrameInputs(frame, nil, 0.001)
	test.That(t, len(restrictRandomInputs), test.ShouldEqual, len(frame.DoF()))
	test.That(t, restrictRandomInputs[0].Value, test.ShouldBeLessThan, 0.03)
	test.That(t, restrictRandomInputs[0].Value, test.ShouldBeGreaterThan, -0.03)
}

func TestRevoluteFrame(t *testing.T) {
	// define a prismatic transform
	axis := r3.Vector{1, 0, 0}                                                    // axis of rotation is x axis
	frame := &rotationalFrame{"test", axis, []Limit{{-math.Pi / 2, math.Pi / 2}}} // limits between -90 and 90 degrees
	// expected output
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, math.Pi/4) // 45 degrees
	// get expected transform back
	input := JointPosToInputs(&pb.ArmJointPositions{Degrees: []float64{45}})
	pose, err := frame.Transform(input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get error back
	input = JointPosToInputs(&pb.ArmJointPositions{Degrees: []float64{45, 55}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in empty input, should get errr back
	input = JointPosToInputs(&pb.ArmJointPositions{Degrees: []float64{}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	overLimit := 100.0 // degrees
	input = JointPosToInputs(&pb.ArmJointPositions{Degrees: []float64{overLimit}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f input out of rev frame bounds %.5f", utils.DegToRad(overLimit), frame.DoF()[0]))
	// gets the correct limits back
	limit := frame.DoF()
	expLimit := []Limit{{Min: -math.Pi / 2, Max: math.Pi / 2}}
	test.That(t, limit, test.ShouldHaveLength, 1)
	test.That(t, limit[0], test.ShouldResemble, expLimit[0])
}

func TestSerializationStatic(t *testing.T) {
	f, err := NewStaticFrame("foo", spatial.NewPoseFromAxisAngle(r3.Vector{1, 2, 3}, r3.Vector{4, 5, 6}, math.Pi/2))
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2, err := UnmarshalFrameJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f.AlmostEquals(f2), test.ShouldBeTrue)
}

func TestSerializationTranslation(t *testing.T) {
	f, err := NewTranslationalFrame("foo", []bool{true, false, true}, []Limit{{1, 2}, {3, 4}})
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2, err := UnmarshalFrameJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f.AlmostEquals(f2), test.ShouldBeTrue)
	test.That(t, f2, test.ShouldResemble, f)
}

func TestSerializationRotations(t *testing.T) {
	f, err := NewRotationalFrame("foo", spatial.R4AA{3.7, 2.1, 3.1, 4.1}, Limit{5, 6})
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2, err := UnmarshalFrameJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f.AlmostEquals(f2), test.ShouldBeTrue)
	test.That(t, f2, test.ShouldResemble, f)
}
