package armplanning

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func nudgeTestFS(t *testing.T) *referenceframe.FrameSystem {
	t.Helper()
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)
	return fs
}

func nudgeLinearInputs(vals []float64) *referenceframe.LinearInputs {
	li := referenceframe.NewLinearInputs()
	li.Put("arm", vals)
	return li
}

func TestNudgePathDirection(t *testing.T) {
	a := nudgeLinearInputs([]float64{0, 0, 0, 0, 0, 0})
	b := nudgeLinearInputs([]float64{1, 0, 2, 0, -2, 0})
	dir := pathDirection(a, b, []string{"arm"})

	norm := 0.0
	for _, d := range dir["arm"] {
		norm += d * d
	}
	test.That(t, math.Sqrt(norm), test.ShouldAlmostEqual, 1.0, 1e-9)
	// Direction should be proportional to b-a.
	test.That(t, dir["arm"][2]/dir["arm"][0], test.ShouldAlmostEqual, 2.0, 1e-9)
	test.That(t, dir["arm"][4]/dir["arm"][0], test.ShouldAlmostEqual, -2.0, 1e-9)
}

func TestNudgeRandomLateralDelta(t *testing.T) {
	fs := nudgeTestFS(t)
	moving := []string{"arm"}
	a := nudgeLinearInputs([]float64{0, 0.5, -1.0, 0, 0.5, 0})
	b := nudgeLinearInputs([]float64{math.Pi / 2, 0.5, -1.0, 0, 0.5, 0})
	dir := pathDirection(a, b, moving)

	rnd := rand.New(rand.NewSource(7))
	const radius = 0.3
	for i := 0; i < 50; i++ {
		delta := randomLateralDelta(fs, moving, radius, dir, rnd)

		// Perpendicular to the path direction.
		dot := 0.0
		maxAbs := 0.0
		for j, d := range delta["arm"] {
			dot += d * dir["arm"][j]
			maxAbs = math.Max(maxAbs, math.Abs(d))
		}
		test.That(t, dot, test.ShouldAlmostEqual, 0.0, 1e-9)
		// Scaled so the largest joint offset is exactly the radius.
		test.That(t, maxAbs, test.ShouldAlmostEqual, radius, 1e-9)
	}
}

func TestNudgeApplyJointDeltaClamps(t *testing.T) {
	fs := nudgeTestFS(t)
	limits := fs.Frame("arm").DoF()

	base := nudgeLinearInputs([]float64{0, limits[1].Max, limits[2].Min, 0, 0, 0})
	delta := jointDelta{"arm": []float64{0.25, 1.0, -1.0, 0, 0, -0.25}}
	out := applyJointDelta(fs, base, delta)

	got := out.Get("arm")
	test.That(t, got[0], test.ShouldAlmostEqual, 0.25)
	test.That(t, got[1], test.ShouldAlmostEqual, limits[1].Max) // clamped
	test.That(t, got[2], test.ShouldAlmostEqual, limits[2].Min) // clamped
	test.That(t, got[5], test.ShouldAlmostEqual, -0.25)

	// A frame absent from the delta map is passed through untouched.
	other := nudgeLinearInputs([]float64{1, 2, 0, 0, 0, 0})
	out = applyJointDelta(fs, other, jointDelta{})
	test.That(t, out.Get("arm")[1], test.ShouldAlmostEqual, 2.0)
}

// TestNudgeSolvesBarelyBlockedPath reproduces the scenario the nudge pass is
// for: the straight-line joint interpolation to the goal grazes a small
// obstacle mid-path, and a slight detour clears it. The plan must succeed via
// the nudge repair, without falling back to a CBiRRT search.
func TestNudgeSolvesBarelyBlockedPath(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := nudgeTestFS(t)
	model := fs.Frame("arm")

	startJoints := []float64{0, 0.5, -1.0, 0, 0.5, 0}
	goalJoints := []float64{math.Pi / 2, 0.5, -1.0, 0, 0.5, 0}
	midJoints := []float64{math.Pi / 4, 0.5, -1.0, 0, 0.5, 0}

	midPose, err := model.Transform(midJoints)
	test.That(t, err, test.ShouldBeNil)
	goalPose, err := model.Transform(goalJoints)
	test.That(t, err, test.ShouldBeNil)

	// A thin post right where the end effector sweeps through mid-path: the
	// straight line is blocked, but a small nudge goes around it.
	obstacle, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(midPose.Point()), r3.Vector{X: 20, Y: 20, Z: 150}, "post")
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
	test.That(t, meta.GoalsNudgeSolved, test.ShouldEqual, 1)
	test.That(t, meta.GoalsCBIRRTSolved, test.ShouldEqual, 0)

	// The trajectory should still land on the goal pose.
	finalPose, err := fs.Transform(
		plan.Trajectory()[len(plan.Trajectory())-1].ToLinearInputs(),
		referenceframe.NewPoseInFrame("arm", spatialmath.NewZeroPose()),
		referenceframe.World,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(finalPose.(*referenceframe.PoseInFrame).Pose(), goalPose, 1), test.ShouldBeTrue)
}
