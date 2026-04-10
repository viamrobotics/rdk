package armplanning

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// TestPlanningWithIntermediateFrame tests that the motion planner can plan for a frame
// attached to an intermediate joint of a flattened model (not the end-effector).
func TestPlanningWithIntermediateFrame(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// Load UR5e arm model
	ur5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "ur5e")
	test.That(t, err, test.ShouldBeNil)

	// Build FS via NewFrameSystem so flattening occurs.
	// Attach a tool to "ur5e:forearm_link" which is after the first 3 joints.
	armLIF := frame.NewLinkInFrame(frame.World, spatialmath.NewZeroPose(), "ur5e", nil)
	toolLIF := frame.NewLinkInFrame("ur5e:forearm_link", spatialmath.NewZeroPose(), "tool", nil)

	parts := []*frame.FrameSystemPart{
		{FrameConfig: armLIF, ModelFrame: ur5e},
	}
	fs, err := frame.NewFrameSystem("test", parts, []*frame.LinkInFrame{toolLIF})
	test.That(t, err, test.ShouldBeNil)

	// Start every joint at 0.5 rad.
	startInputs := frame.NewZeroInputs(fs)
	startInputs["ur5e"] = []frame.Input{0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
	startLI := startInputs.ToLinearInputs()

	// Goal: move the first 3 joints to 0.6 rad (these are the joints that affect the tool).
	// This guarantees reachability since only joints before forearm_link matter.
	goalInputs := frame.NewZeroInputs(fs)
	goalInputs["ur5e"] = []frame.Input{0.6, 0.6, 0.6, 0.5, 0.5, 0.5}
	goalLI := goalInputs.ToLinearInputs()

	// Compute the tool's pose at the goal configuration — this is our planning target.
	toolPIF := frame.NewPoseInFrame("tool", spatialmath.NewZeroPose())
	goalResult, err := fs.Transform(goalLI, toolPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	goalPose := goalResult.(*frame.PoseInFrame).Pose()
	t.Logf("tool at goal config: %v", goalPose.Point())

	// Also log the start for comparison.
	startResult, err := fs.Transform(startLI, toolPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	t.Logf("tool at start config: %v", startResult.(*frame.PoseInFrame).Pose().Point())

	goal := &PlanState{poses: frame.FrameSystemPoses{
		"tool": frame.NewPoseInFrame(frame.World, goalPose),
	}}

	plan, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{goal},
		StartState:     &PlanState{structuredConfiguration: startInputs},
		PlannerOptions: NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan.Trajectory()), test.ShouldBeGreaterThanOrEqualTo, 2)
}
