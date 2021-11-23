package motionplan

import (
	"context"
	"testing"

	commonpb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/varm/v1.json"), "")
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

	modelXarm, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm6_kinematics.json"), "")
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
	ok, failCI = handler.CheckConstraintPath(ciBad, 0.005)
	test.That(t, failCI, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}
