package armplanning

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

func TestUnreachableGoalNamesBlockingJoint(t *testing.T) {
	logger := logging.NewTestLogger(t)

	base, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	simple, ok := base.(*frame.SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	limited, err := frame.NewModelWithLimitOverrides(simple, map[string]frame.Limit{
		"wrist_3_joint": {Min: 0, Max: utils.DegToRad(90)},
	})
	test.That(t, err, test.ShouldBeNil)

	fs := frame.NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(limited, fs.World()), test.ShouldBeNil)

	// Only wrist 3 differs between start and goal, and the goal needs it at 150°, outside [0°, 90°].
	start := []frame.Input{0, -1.2, 1.5, -1.9, -1.6, utils.DegToRad(45)}
	goalConfig := append([]frame.Input{}, start...)
	goalConfig[5] = utils.DegToRad(150)
	goalPose, err := base.Transform(goalConfig)
	test.That(t, err, test.ShouldBeNil)

	_, _, err = PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{{poses: frame.FrameSystemPoses{
			limited.Name(): frame.NewPoseInFrame(frame.World, goalPose),
		}}},
		StartState:     &PlanState{structuredConfiguration: frame.FrameSystemInputs{limited.Name(): start}},
		PlannerOptions: NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, errors.Is(err, errIKSolve), test.ShouldBeTrue)

	var jle *jointLimitError
	test.That(t, errors.As(err, &jle), test.ShouldBeTrue)
	test.That(t, len(jle.blocked), test.ShouldEqual, 1)
	test.That(t, jle.blocked[0].joint, test.ShouldEqual, "wrist_3_joint")
	test.That(t, jle.blocked[0].units, test.ShouldEqual, "°")
	test.That(t, jle.blocked[0].max, test.ShouldAlmostEqual, 90, 1e-6)
	// Several arm configurations reach the goal pose, and they need wrist 3 on different sides of
	// its range, so only assert that the reported value is outside it.
	needed := jle.blocked[0].needed
	test.That(t, needed < 0 || needed > 90, test.ShouldBeTrue)
}
