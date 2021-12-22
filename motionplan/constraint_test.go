package motionplan

import (
	"context"
	"testing"

	commonpb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	vutils "go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var logger = golog.NewDevelopmentLogger("armplay")

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseJSONFile(vutils.ResolveFile("robots/varm/v1.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mp, err := NewCBiRRTMotionPlanner(m, nCPU, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	pos := &commonpb.Pose{
		X:  -46,
		Y:  0,
		Z:  372,
		OX: -1.78,
		OY: -3.3,
		OZ: -1.11,
	}
	_, err = mp.Plan(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	opt := NewDefaultPlannerOptions()
	opt.SetMetric(NewPositionOnlyMetric())
	opt.SetMaxSolutions(50)
	mp.SetOptions(opt)
	_, err = mp.Plan(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldBeNil)
}

func TestConstraintPath(t *testing.T) {

	homePos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	toPos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 1})

	modelXarm, err := frame.ParseJSONFile(vutils.ResolveFile("robots/xarm/xArm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ci := &ConstraintInput{StartInput: homePos, EndInput: toPos, Frame: modelXarm}

	handler := &constraintHandler{}

	// No constraints, should pass
	ok, failCI := handler.CheckConstraintPath(ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	// Test interpolating
	handler.AddConstraint("interp", NewInterpolatingConstraint(0.5))
	ok, failCI = handler.CheckConstraintPath(ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, len(handler.Constraints()), test.ShouldEqual, 1)

	badInterpPos := frame.FloatsToInputs([]float64{6.2, 0, 0, 0, 0, 0})
	ciBad := &ConstraintInput{StartInput: homePos, EndInput: badInterpPos, Frame: modelXarm}
	ok, failCI = handler.CheckConstraintPath(ciBad, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestLineFollow(t *testing.T) {
	p1 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  440,
		Y:  -447,
		Z:  500,
		OY: -1,
	})
	p2 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  140,
		Y:  -447,
		Z:  550,
		OY: -1,
	})
	mp1 := frame.JointPositionsFromRadians([]float64{
		3.75646398939225,
		-1.0162453766159272,
		1.2142890600914453,
		1.0521227724322786,
		-0.21337105357552288,
		-0.006502311329196852,
		-4.3822913510408945,
	})
	mp2 := frame.JointPositionsFromRadians([]float64{
		3.896845654143853,
		-0.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	})

	query := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  289.94907586421124,
		Y:  -447,
		Z:  525.0086401700755,
		OY: -1,
	})

	validOV := &spatial.OrientationVector{OY: -1}
	validFunc, gradFunc := NewLineConstraintAndGradient(p1.Point(), p2.Point(), validOV, 0., 0.001)

	pointGrad := gradFunc(query, query)
	test.That(t, pointGrad, test.ShouldBeLessThan, 0.001*0.001)

	fs := frame.NewEmptySimpleFrameSystem("test")

	m, err := frame.ParseJSONFile(vutils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 105}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	fss := NewSolvableFrameSystem(fs, logger)

	solveFrame := markerFrame
	goalFrame := fs.World()

	sFrames, err := fss.TracebackFrame(solveFrame)
	test.That(t, err, test.ShouldBeNil)
	gFrames, err := fss.TracebackFrame(goalFrame)
	test.That(t, err, test.ShouldBeNil)
	frames := uniqInPlaceSlice(append(sFrames, gFrames...))

	// Create a frame to solve for, and an IK solver with that frame.
	sf := &solverFrame{solveFrame.Name() + "_" + goalFrame.Name(), fss, frames, solveFrame, goalFrame}

	opt := NewDefaultPlannerOptions()
	opt.SetPathDist(gradFunc)
	opt.AddConstraint("whiteboard", validFunc)
	ok, _ := opt.CheckConstraintPath(
		&ConstraintInput{
			StartInput: frame.JointPosToInputs(mp1),
			EndInput:   frame.JointPosToInputs(mp2),
			Frame:      sf,
		},
		1,
	)

	test.That(t, ok, test.ShouldBeFalse)
}
