package referenceframe

import (
	"context"
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
	ctx := context.Background()
	// define a static transform
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{1, 2, 3}, r3.Vector{0, 0, 1}, math.Pi/2)
	frame := &staticFrame{"test", expPose}
	// get expected transform back
	emptyInput := FloatsToInputs([]float64{})
	pose, err := frame.Transform(ctx, emptyInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in non-empty input, should get err back
	nonEmptyInput := FloatsToInputs([]float64{0, 0, 0})
	_, err = frame.Transform(ctx, nonEmptyInput)
	test.That(t, err, test.ShouldNotBeNil)
	// check that there are no limits on the static frame
	limits := frame.DoF(ctx)
	test.That(t, limits, test.ShouldResemble, []*pb.Limit{})

	errExpect := errors.New("pose is not allowed to be nil")
	f, err := NewStaticFrame("test2", nil)
	test.That(t, err.Error(), test.ShouldEqual, errExpect.Error())
	test.That(t, f, test.ShouldBeNil)
}

func TestPrismaticFrame(t *testing.T) {
	ctx := context.Background()
	// define a prismatic transform
	limits := []*pb.Limit{{Min: -30, Max: 30}}
	frame, err := NewTranslationalFrame("test", []bool{false, true, false}, limits) // can only move on y axis
	test.That(t, err, test.ShouldBeNil)
	// this should return an error
	badLimits := []*pb.Limit{{Min: 0, Max: 0}, {Min: -30, Max: 30}, {Min: 0, Max: 0}}
	_, err = NewTranslationalFrame("test", []bool{false, true, false}, badLimits) // can only move on y axis
	test.That(t, err, test.ShouldBeError, errors.New("given number of limits 3 does not match number of axes 1"))
	// expected output
	expPose := spatial.NewPoseFromPoint(r3.Vector{0, 20, 0})
	// get expected transform back
	input := FloatsToInputs([]float64{20})
	pose, err := frame.Transform(ctx, input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get an error back
	input = FloatsToInputs([]float64{0, 20, 0})
	_, err = frame.Transform(ctx, input)
	test.That(t, err, test.ShouldNotBeNil)

	// if you feed in empty input, should get an error
	input = FloatsToInputs([]float64{})
	_, err = frame.Transform(ctx, input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	overLimit := 50.0
	input = FloatsToInputs([]float64{overLimit})
	_, err = frame.Transform(ctx, input)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f input out of bounds %v", overLimit, frame.DoF(ctx)[0]))
	// gets the correct limits back
	frameLimits := frame.DoF(ctx)
	test.That(t, frameLimits, test.ShouldResemble, limits)
}

func TestRevoluteFrame(t *testing.T) {
	ctx := context.Background()
	// define a prismatic transform
	axis := spatial.R4AA{RX: 1, RY: 0, RZ: 0}                                               // axis of rotation is x axis
	frame := &rotationalFrame{"test", axis, &pb.Limit{Min: -math.Pi / 2, Max: math.Pi / 2}} // limits between -90 and 90 degrees
	// expected output
	expPose := spatial.NewPoseFromAxisAngle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, math.Pi/4) // 45 degrees
	// get expected transform back
	input := JointPosToInputs(&pb.JointPositions{Degrees: []float64{45}})
	pose, err := frame.Transform(ctx, input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get error back
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{45, 55}})
	_, err = frame.Transform(ctx, input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in empty input, should get errr back
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{}})
	_, err = frame.Transform(ctx, input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	overLimit := 100.0 // degrees
	input = JointPosToInputs(&pb.JointPositions{Degrees: []float64{overLimit}})
	_, err = frame.Transform(ctx, input)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f input out of rev frame bounds %v", utils.DegToRad(overLimit), frame.DoF(ctx)[0]))
	// gets the correct limits back
	limit := frame.DoF(ctx)
	expLimit := []*pb.Limit{&pb.Limit{Min: -math.Pi / 2, Max: math.Pi / 2}}
	test.That(t, limit, test.ShouldHaveLength, 1)
	test.That(t, limit[0].String(), test.ShouldResemble, expLimit[0].String())
}

func TestBasicConversions(t *testing.T) {
	jp := &pb.JointPositions{Degrees: []float64{45, 55}}
	inputs := JointPosToInputs(jp)
	test.That(t, jp, test.ShouldResemble, InputsToJointPos(inputs))

	floats := []float64{45, 55, 27}
	inputs = FloatsToInputs(floats)
	test.That(t, floats, test.ShouldResemble, InputsToFloats(inputs))
}
