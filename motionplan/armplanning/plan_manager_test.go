package armplanning

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// TestPlanMotionJointGoalAlreadyAtTarget covers the no-op case where every
// actuated frame's joint goal already matches the start configuration.
func TestPlanMotionJointGoalAlreadyAtTarget(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	joints := []float64{0, 0.5, -1.0, 0, 0.5, 0}

	req := &PlanRequest{
		FrameSystem:    fs,
		StartState:     NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": joints}),
		Goals:          []*PlanState{NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": joints})},
		PlannerOptions: NewBasicPlannerOptions(),
	}

	plan, _, err := PlanMotion(ctx, logger, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)

	traj := plan.Trajectory()
	test.That(t, traj, test.ShouldHaveLength, 1)
	test.That(t, traj[0]["arm"], test.ShouldResemble, joints)
}
