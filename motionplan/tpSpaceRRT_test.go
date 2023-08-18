//go:build !windows

package motionplan

import (
	"context"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const testTurnRad = 0.3

func TestPtgRrt(t *testing.T) {
	logger := golog.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}

	ackermanFrame, err := referenceframe.NewPTGFrameFromTurningRadius(
		"ackframe",
		300.,
		testTurnRad,
		0,
		geometries,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPose(r3.Vector{X: 50, Y: 10, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180})

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.SetGoalMetric(NewPositionOnlyMetric(goalPos))
	opt.DistanceFunc = SquaredNormNoOrientSegmentMetric
	opt.GoalThreshold = 10.
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, ok := mp.(*tpSpaceRRTMotionPlanner)
	test.That(t, ok, test.ShouldBeTrue)

	plan, err := tp.plan(context.Background(), goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThanOrEqualTo, 2)
}

func TestPtgWithObstacle(t *testing.T) {
	logger := golog.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}
	ackermanFrame, err := referenceframe.NewPTGFrameFromTurningRadius(
		"ackframe",
		300.,
		testTurnRad,
		0,
		geometries,
	)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.SetGoalMetric(NewPositionOnlyMetric(goalPos))
	opt.DistanceFunc = SquaredNormNoOrientSegmentMetric
	opt.GoalThreshold = 30.
	// obstacles
	obstacle1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, -500, 0}), r3.Vector{180, 1800, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 2000, 0}), r3.Vector{180, 1800, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle3, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, -1400, 0}), r3.Vector{50000, 30, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle4, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 2400, 0}), r3.Vector{50000, 30, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	geoms := []spatialmath.Geometry{obstacle1, obstacle2, obstacle3, obstacle4}

	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, ackermanFrame.Name(), referenceframe.World, nil)
	test.That(t, err, test.ShouldBeNil)
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, worldState, referenceframe.StartPositions(fs), nil)
	test.That(t, err, test.ShouldBeNil)

	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	plan, err := tp.plan(ctx, goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
}

func TestPtgCheckPlan(t *testing.T) {
	logger := golog.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}
	ackermanFrame, err := referenceframe.NewPTGFrameFromTurningRadius(
		"ackframe",
		300.,
		testTurnRad,
		0,
		geometries,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.SetGoalMetric(NewPositionOnlyMetric(goalPos))
	opt.DistanceFunc = SquaredNormNoOrientSegmentMetric
	opt.GoalThreshold = 30.

	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	plan, err := tp.plan(context.Background(), goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	planAsInputs := nodesToInputs(plan)

	t.Run("obstacles blocking path", func(t *testing.T) {
		// create obstacle blocking path
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{1000, 0, 0}), r3.Vector{20, 20, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		valid, err := CheckPlan(ackermanFrame, planAsInputs, gifs, fs)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, valid, test.ShouldBeFalse)
	})

	t.Run("obstacles blocking path2", func(t *testing.T) {
		// create obstacle blocking path
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2000, 0, 0}), r3.Vector{20, 20, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		valid, err := CheckPlan(ackermanFrame, planAsInputs, gifs, fs)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, valid, test.ShouldBeFalse)
	})

	t.Run("obstacles blocking path2.5", func(t *testing.T) {
		// create obstacle blocking path
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 0, 0}), r3.Vector{20, 20, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		valid, err := CheckPlan(ackermanFrame, planAsInputs, gifs, fs)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, valid, test.ShouldBeFalse)
	})

	t.Run("obstacles blocking path3", func(t *testing.T) {
		// create obstacle blocking path
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{3000, 0, 0}), r3.Vector{20, 20, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		valid, err := CheckPlan(ackermanFrame, planAsInputs, gifs, fs)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, valid, test.ShouldBeFalse)
	})

	// create camera_origin frame
	cameraOriginFrame, err := referenceframe.NewStaticFrame("camera-origin", spatialmath.NewPoseFromPoint(r3.Vector{0, -20, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(cameraOriginFrame, ackermanFrame)
	test.That(t, err, test.ShouldBeNil)

	// create camera geometry
	cameraGeom, err := spatialmath.NewBox(
		spatialmath.NewZeroPose(),
		r3.Vector{1, 1, 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)

	// create cameraFrame and add to framesystem
	cameraFrame, err := referenceframe.NewStaticFrameWithGeometry(
		"camera-frame", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}), cameraGeom,
	)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(cameraFrame, cameraOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	t.Run("obstacles NOT in world frame - no collision", func(t *testing.T) {
		// create obstacle
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{2500, 0, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		valid, err := CheckPlan(ackermanFrame, planAsInputs, gifs, fs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, valid, test.ShouldBeTrue)
	})

	t.Run("obstacles NOT in world frame - yes collision", func(t *testing.T) {
		// create obstacle
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{2500, 20, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		valid, err := CheckPlan(ackermanFrame, planAsInputs, gifs, fs)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, valid, test.ShouldBeFalse)
	})
}
