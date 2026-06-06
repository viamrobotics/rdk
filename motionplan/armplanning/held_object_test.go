package armplanning

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// heldObjectTestFS builds a frame system with a single 6-DoF arm and returns it
// along with the arm model frame.
func heldObjectTestFS(t *testing.T) (*frame.FrameSystem, frame.Frame) {
	t.Helper()
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)
	return fs, model
}

// TestValidatePlanRequestMergesWorldStateTransforms verifies finding #1/#2: a
// WorldState transform (LinkInFrame) handed to the direct planner is merged into
// the planning frame system (on a copy, leaving the caller's FS untouched), and a
// transform that cannot be applied surfaces a loud error instead of being silently
// dropped.
func TestValidatePlanRequestMergesWorldStateTransforms(t *testing.T) {
	fs, model := heldObjectTestFS(t)
	heldBox, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 60, Y: 60, Z: 60}, "heldObject")
	test.That(t, err, test.ShouldBeNil)

	goal := frame.NewPoseInFrame(frame.World, spatialmath.NewPoseFromPoint(r3.Vector{X: 400, Z: 200}))
	newReq := func(ws *frame.WorldState) *PlanRequest {
		return &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{poses: frame.FrameSystemPoses{model.Name(): goal}}},
			StartState:     &PlanState{structuredConfiguration: frame.NewNeutralFrameSystemInputs(fs)},
			WorldState:     ws,
			PlannerOptions: NewBasicPlannerOptions(),
		}
	}

	t.Run("transform is merged into a copy of the frame system", func(t *testing.T) {
		heldLIF := frame.NewLinkInFrame(model.Name(), spatialmath.NewZeroPose(), "heldObject", heldBox)
		ws, err := frame.NewWorldState(nil, []*frame.LinkInFrame{heldLIF})
		test.That(t, err, test.ShouldBeNil)

		req := newReq(ws)
		test.That(t, req.validatePlanRequest(), test.ShouldBeNil)

		// The transform now exists as a real frame in the request's frame system...
		test.That(t, req.FrameSystem.Frame("heldObject"), test.ShouldNotBeNil)
		// ...and the original frame system the caller passed in is untouched, so it can be reused.
		test.That(t, fs.Frame("heldObject"), test.ShouldBeNil)
	})

	t.Run("a transform with a missing parent fails loudly", func(t *testing.T) {
		orphan := frame.NewLinkInFrame("noSuchFrame", spatialmath.NewZeroPose(), "orphan", heldBox)
		ws, err := frame.NewWorldState(nil, []*frame.LinkInFrame{orphan})
		test.That(t, err, test.ShouldBeNil)

		err = newReq(ws).validatePlanRequest()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "transform")
	})
}

// TestPlanMotionHeldObjectCollisionCheckedAlongTrajectory verifies that a box attached
// to the moving arm via a WorldState transform is collision-checked at the goal configuration
// (because it is a real frame that moves with the arm), whereas the same box expressed as a
// WorldState obstacle in the arm frame is frozen at the start configuration and the
// goal-configuration collision goes undetected.
func TestPlanMotionHeldObjectCollisionCheckedAlongTrajectory(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs, model := heldObjectTestFS(t)

	startInputs := frame.NewNeutralFrameSystemInputs(fs)
	// A configuration that swings the arm well away from neutral so the held object's world
	// location at the goal is far from where it sits at the start.
	goalInputs := frame.FrameSystemInputs{model.Name(): {1.2, -1.0, -1.4, 0, 0.6, 0}}

	// The held object: a box 200mm beyond the tool tip, attached to the (moving) arm frame.
	heldOffset := spatialmath.NewPoseFromPoint(r3.Vector{Z: 200})
	heldDims := r3.Vector{X: 80, Y: 80, Z: 80}
	heldBox, err := spatialmath.NewBox(spatialmath.NewZeroPose(), heldDims, "heldObject")
	test.That(t, err, test.ShouldBeNil)

	heldWorldPose := func(inputs frame.FrameSystemInputs) spatialmath.Pose {
		t.Helper()
		tf, err := fs.Transform(inputs.ToLinearInputs(), frame.NewPoseInFrame(model.Name(), heldOffset), frame.World)
		test.That(t, err, test.ShouldBeNil)
		return tf.(*frame.PoseInFrame).Pose()
	}

	heldAtStart := heldWorldPose(startInputs)
	heldAtGoal := heldWorldPose(goalInputs)

	// Sanity-check the fixture: the held object must travel a meaningful distance between start
	// and goal, otherwise an obstacle placed at the goal would already overlap it at the start and
	// the test would not distinguish "checked along the trajectory" from "frozen at the start".
	separation := heldAtStart.Point().Sub(heldAtGoal.Point()).Norm()
	test.That(t, separation, test.ShouldBeGreaterThan, 300)

	// A static wall placed exactly where the held object ends up at the goal configuration.
	wall, err := spatialmath.NewBox(heldAtGoal, heldDims, "wall")
	test.That(t, err, test.ShouldBeNil)
	wallGIF := frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{wall})

	planToGoalConfig := func(ws *frame.WorldState) error {
		_, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{structuredConfiguration: goalInputs}},
			StartState:     &PlanState{structuredConfiguration: startInputs},
			WorldState:     ws,
			PlannerOptions: NewBasicPlannerOptions(),
		})
		return err
	}

	t.Run("bare arm reaches the goal configuration", func(t *testing.T) {
		// The wall sits beyond the tool tip, in free space, so the arm itself does not hit it.
		ws, err := frame.NewWorldState([]*frame.GeometriesInFrame{wallGIF}, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, planToGoalConfig(ws), test.ShouldBeNil)
	})

	t.Run("held object as a transform is collision-checked at the goal", func(t *testing.T) {
		heldLIF := frame.NewLinkInFrame(model.Name(), heldOffset, "heldObject", heldBox)
		ws, err := frame.NewWorldState([]*frame.GeometriesInFrame{wallGIF}, []*frame.LinkInFrame{heldLIF})
		test.That(t, err, test.ShouldBeNil)
		// The held object collides with the wall at the goal configuration, so the plan must fail.
		test.That(t, planToGoalConfig(ws), test.ShouldNotBeNil)
	})

	t.Run("a WorldState obstacle in a moving frame is frozen at the start", func(t *testing.T) {
		// Contrast with the transform above: expressing the held object as a WorldState obstacle in
		// the (moving) arm frame does NOT track the arm. The planner resolves WorldState obstacles to
		// the world frame exactly once, at the start configuration, so the obstacle stays where the
		// arm was at the start of the move - nowhere near where the real held object ends up.
		obsBox, err := spatialmath.NewBox(heldOffset, heldDims, "heldAsObstacle")
		test.That(t, err, test.ShouldBeNil)
		ws, err := frame.NewWorldState(
			[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(model.Name(), []spatialmath.Geometry{obsBox})}, nil)
		test.That(t, err, test.ShouldBeNil)

		// A WorldState obstacle does not track the arm's joint configuration at all: resolving it at
		// the start and at the goal yields the same world location (the geometry is effectively pinned
		// to the base, not the moving end-effector). On top of that, the planner only ever resolves
		// WorldState obstacles once, at the start configuration (see constraint_checker.go). Either way
		// the obstacle never follows the arm to the goal, which is why a held object cannot be modeled
		// as a WorldState obstacle and must instead be a transform (as exercised above).
		atStart, err := ws.ObstaclesInWorldFrame(fs, startInputs)
		test.That(t, err, test.ShouldBeNil)
		atGoal, err := ws.ObstaclesInWorldFrame(fs, goalInputs)
		test.That(t, err, test.ShouldBeNil)
		startPoint := atStart.Geometries()[0].Pose().Point()
		goalPoint := atGoal.Geometries()[0].Pose().Point()
		test.That(t, startPoint.Sub(goalPoint).Norm(), test.ShouldBeLessThan, 1e-6)
	})
}

// TestPlanMotionHeldObjectCollidesBetweenEndpoints demonstates that
// a held object that is collision-free at BOTH the start and the goal configurations, yet collides
// with the environment partway along the move. This is the case the previous (frozen-at-seed)
// modeling could never catch and a goal-only check would miss: only a planner that carries the held
// geometry with the arm and checks it at every state along the trajectory detects it.
//
// The fixture is a doorway - two walls with a gap between them. The arm sweeps from one side to the
// other; the gap is wide enough for the (thin) arm, but the arm is carrying a large box that is too
// wide to fit through and so strikes the walls midway through the swing.
func TestPlanMotionHeldObjectCollidesBetweenEndpoints(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs, model := heldObjectTestFS(t)

	// A waist sweep with the shoulder pitched forward: the arm (and the box it holds) swings through
	// an arc, reaching furthest out in +X at the midpoint and tucked back at both endpoints.
	startInputs := frame.FrameSystemInputs{model.Name(): {-1.3, -1.5, 0, 0, 0, 0}}
	goalInputs := frame.FrameSystemInputs{model.Name(): {1.3, -1.5, 0, 0, 0, 0}}
	midInputs := frame.FrameSystemInputs{model.Name(): {0, -1.5, 0, 0, 0, 0}}

	// The held object: a large box carried 200mm beyond the tool tip.
	heldOffset := spatialmath.NewPoseFromPoint(r3.Vector{Z: 200})
	heldDims := r3.Vector{X: 220, Y: 220, Z: 220}
	heldBox, err := spatialmath.NewBox(spatialmath.NewZeroPose(), heldDims, "heldObject")
	test.That(t, err, test.ShouldBeNil)

	heldGeometryAt := func(inputs frame.FrameSystemInputs) spatialmath.Geometry {
		t.Helper()
		tf, err := fs.Transform(inputs.ToLinearInputs(), frame.NewPoseInFrame(model.Name(), heldOffset), frame.World)
		test.That(t, err, test.ShouldBeNil)
		g, err := spatialmath.NewBox(tf.(*frame.PoseInFrame).Pose(), heldDims, "heldCheck")
		test.That(t, err, test.ShouldBeNil)
		return g
	}

	// Build the doorway around where the held object passes at the midpoint of the swing.
	heldMid := heldGeometryAt(midInputs).Pose().Point()
	wallDims := r3.Vector{X: 120, Y: 200, Z: 400}
	leftWall, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(r3.Vector{X: heldMid.X, Y: heldMid.Y + 170, Z: heldMid.Z}), wallDims, "leftWall")
	test.That(t, err, test.ShouldBeNil)
	rightWall, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(r3.Vector{X: heldMid.X, Y: heldMid.Y - 170, Z: heldMid.Z}), wallDims, "rightWall")
	test.That(t, err, test.ShouldBeNil)
	walls := []spatialmath.Geometry{leftWall, rightWall}
	wallsGIF := frame.NewGeometriesInFrame(frame.World, walls)

	// Confirm the fixture really has the shape the test relies on: the held object is clear of both
	// walls at the start and the goal, but collides with the doorway in the middle of the move.
	collidesWithAWall := func(g spatialmath.Geometry) bool {
		t.Helper()
		for _, w := range walls {
			collides, _, err := g.CollidesWith(w, 0)
			test.That(t, err, test.ShouldBeNil)
			if collides {
				return true
			}
		}
		return false
	}
	test.That(t, collidesWithAWall(heldGeometryAt(startInputs)), test.ShouldBeFalse)
	test.That(t, collidesWithAWall(heldGeometryAt(goalInputs)), test.ShouldBeFalse)
	test.That(t, collidesWithAWall(heldGeometryAt(midInputs)), test.ShouldBeTrue)

	t.Run("bare arm passes through the doorway", func(t *testing.T) {
		// Without the held object the arm is thin enough to swing through the gap, so the move succeeds.
		ws, err := frame.NewWorldState([]*frame.GeometriesInFrame{wallsGIF}, nil)
		test.That(t, err, test.ShouldBeNil)
		_, _, err = PlanMotion(context.Background(), logger, &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{structuredConfiguration: goalInputs}},
			StartState:     &PlanState{structuredConfiguration: startInputs},
			WorldState:     ws,
			PlannerOptions: NewBasicPlannerOptions(),
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("held object is collision-checked between the endpoints", func(t *testing.T) {
		// Attach the large box to the arm via a WorldState transform and build the plan the way
		// PlanMotion does: validatePlanRequest merges the transform into the frame system, then we
		// construct the same constraint checker the planner uses for the start->goal segment.
		heldLIF := frame.NewLinkInFrame(model.Name(), heldOffset, "heldObject", heldBox)
		ws, err := frame.NewWorldState([]*frame.GeometriesInFrame{wallsGIF}, []*frame.LinkInFrame{heldLIF})
		test.That(t, err, test.ShouldBeNil)

		req := &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{structuredConfiguration: goalInputs}},
			StartState:     &PlanState{structuredConfiguration: startInputs},
			WorldState:     ws,
			PlannerOptions: NewBasicPlannerOptions(),
		}
		test.That(t, req.validatePlanRequest(), test.ShouldBeNil)

		ctx := context.Background()
		pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
		test.That(t, err, test.ShouldBeNil)
		goalPoses, err := req.Goals[0].ComputePoses(ctx, req.FrameSystem)
		test.That(t, err, test.ShouldBeNil)
		start := req.StartState.LinearConfiguration()
		goal := req.Goals[0].LinearConfiguration()
		psc, err := NewPlanSegmentContext(ctx, pc, start, goalPoses)
		test.That(t, err, test.ShouldBeNil)

		// Each endpoint on its own is collision-free...
		_, err = psc.Checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{Configuration: start, FS: req.FrameSystem})
		test.That(t, err, test.ShouldBeNil)
		_, err = psc.Checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{Configuration: goal, FS: req.FrameSystem})
		test.That(t, err, test.ShouldBeNil)

		// ...but the path between them is not, because the held object strikes the doorway midway.
		err = psc.CheckPath(ctx, start, goal, true, nil)
		test.That(t, err, test.ShouldNotBeNil)

		// Guard against a false positive: with the doorway removed, the very same held-object path is
		// collision-free. This proves the failure above is caused by the walls mid-trajectory, not by
		// the held box self-colliding with the arm or anything else along the way.
		wsNoWalls, err := frame.NewWorldState(nil, []*frame.LinkInFrame{heldLIF})
		test.That(t, err, test.ShouldBeNil)
		reqNoWalls := &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{structuredConfiguration: goalInputs}},
			StartState:     &PlanState{structuredConfiguration: startInputs},
			WorldState:     wsNoWalls,
			PlannerOptions: NewBasicPlannerOptions(),
		}
		test.That(t, reqNoWalls.validatePlanRequest(), test.ShouldBeNil)
		pcNoWalls, err := NewPlanContext(ctx, logger, reqNoWalls, &PlanMeta{})
		test.That(t, err, test.ShouldBeNil)
		pscNoWalls, err := NewPlanSegmentContext(ctx, pcNoWalls, start, goalPoses)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pscNoWalls.CheckPath(ctx, start, goal, true, nil), test.ShouldBeNil)
	})
}
