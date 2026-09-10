package armplanning

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestJointGoalDetourKeepsUnchangedArmStill(t *testing.T) {
	for _, moveBoth := range []bool{false, true} {
		name := "one arm changes"
		if moveBoth {
			name = "both arms change"
		}
		t.Run(name, func(t *testing.T) { testJointGoalDetour(t, moveBoth) })
	}
}

func testJointGoalDetour(t *testing.T, moveBoth bool) {
	t.Helper()
	fs, startJoints, goalJoints, req := nudgeBlockedScene(t)
	idle, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "idle")
	if err != nil {
		t.Fatal(err)
	}
	mount, err := referenceframe.NewStaticFrame("idle-mount", spatialmath.NewPoseFromPoint(r3.Vector{X: 2000}))
	if err != nil {
		t.Fatal(err)
	}
	if err = fs.AddFrame(mount, fs.World()); err != nil {
		t.Fatal(err)
	}
	if err = fs.AddFrame(idle, mount); err != nil {
		t.Fatal(err)
	}
	req.StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": startJoints, "idle": startJoints})
	idleGoal := startJoints
	if moveBoth {
		idleGoal = goalJoints
	}
	req.Goals = []*PlanState{NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": goalJoints, "idle": idleGoal})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	plan, _, err := PlanMotion(ctx, logging.NewTestLogger(t), req)
	if err != nil {
		t.Fatal(err)
	}
	trajectory := plan.Trajectory()
	if len(trajectory) < 3 {
		t.Fatalf("expected a detour around the post, got %d waypoints", len(trajectory))
	}
	for i, step := range trajectory {
		if !moveBoth && !slices.Equal(step["idle"], startJoints) {
			t.Fatalf("waypoint %d moves the unchanged arm: %v, want %v", i, step["idle"], startJoints)
		}
	}
	if !slices.Equal(trajectory[len(trajectory)-1]["arm"], goalJoints) {
		t.Fatal("acting arm did not reach its joint goal")
	}
	if !slices.Equal(trajectory[len(trajectory)-1]["idle"], idleGoal) {
		t.Fatal("second arm did not reach its joint goal")
	}
}
