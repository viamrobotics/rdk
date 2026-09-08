package armplanning

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// TestWallHopWithoutSearch: the straight path to a same-joint-family goal is
// blocked by a wall large enough that the nudge cannot skirt it; the cheap
// ladder (roadmap et al.) must solve it without falling back to a cBiRRT
// search.
func TestWallHopWithoutSearch(t *testing.T) {
	logger := logging.NewTestLogger(t)
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	startJoints := []float64{0, 0.5, -1.0, 0, 0.5, 0}
	goalJoints := []float64{math.Pi / 2, 0.5, -1.0, 0, 0.5, 0}
	midJoints := []float64{math.Pi / 4, 0.5, -1.0, 0, 0.5, 0}

	midPose, err := model.Transform(midJoints)
	test.That(t, err, test.ShouldBeNil)
	goalPose, err := model.Transform(goalJoints)
	test.That(t, err, test.ShouldBeNil)

	// A wall across the sweep, wide and deep enough that lateral nudges can't
	// get around it; the only way over is a real up-and-over keypoint hop.
	wallCenter := midPose.Point()
	wallCenter.Z -= 100
	obstacle, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(wallCenter), r3.Vector{X: 350, Y: 350, Z: 500}, "bigwall")
	test.That(t, err, test.ShouldBeNil)

	plan, meta, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			{poses: referenceframe.FrameSystemPoses{"arm": referenceframe.NewPoseInFrame(referenceframe.World, goalPose)}},
		},
		StartState:            &PlanState{structuredConfiguration: referenceframe.FrameSystemInputs{"arm": startJoints}},
		ObstaclesInWorldFrame: referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{obstacle}),
		PlannerOptions:        NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldBeNil)
	t.Logf("nudge=%d roadmap=%d cbirrt=%d traj=%d",
		meta.GoalsNudgeSolved, meta.GoalsRoadmapSolved, meta.GoalsCBIRRTSolved, len(plan.Trajectory()))
	// The point of the scene: no random search should be needed.
	test.That(t, meta.GoalsCBIRRTSolved, test.ShouldEqual, 0)
}
