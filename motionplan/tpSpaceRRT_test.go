//go:build !windows

package motionplan

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var printPath = false

const testTurnRad = 0.3

func TestPtgRrtBidirectional(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}

	ctx := context.Background()

	ackermanFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"ackframe",
		logger,
		300.,
		0,
		testTurnRad,
		0,
		0,
		geometries,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPose(r3.Vector{X: 200, Y: 7000, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90})

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, ok := mp.(*tpSpaceRRTMotionPlanner)
	tp.algOpts.pathdebug = printPath
	if tp.algOpts.pathdebug {
		tp.logger.Debug("$type,X,Y")
		tp.logger.Debugf("$SG,%f,%f\n", 0., 0.)
		tp.logger.Debugf("$SG,%f,%f\n", goalPos.Point().X, goalPos.Point().Y)
	}
	test.That(t, ok, test.ShouldBeTrue)
	plan, err := tp.plan(ctx, goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThanOrEqualTo, 2)

	allPtgs := ackermanFrame.(tpspace.PTGProvider).PTGSolvers()
	lastPose := spatialmath.NewZeroPose()

	if tp.algOpts.pathdebug {
		for _, mynode := range plan {
			trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				if i == 0 {
					tp.logger.Debugf("$WP,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				}
				tp.logger.Debugf("$FINALPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if i == len(trajPts)-1 {
					lastPose = intPose
					break
				}
			}
		}
	}
	tp.planOpts.SmoothIter = 20
	plan = tp.smoothPath(ctx, plan)
	if tp.algOpts.pathdebug {
		lastPose = spatialmath.NewZeroPose()
		for _, mynode := range plan {
			trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				if i == 0 {
					tp.logger.Debugf("$SMOOTHWP,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				}
				tp.logger.Debugf("$SMOOTHPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if pt.Dist >= mynode.Q()[2].Value {
					lastPose = intPose
					break
				}
			}
		}
	}
}

func TestPtgRrtUnidirectional(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}

	ctx := context.Background()

	ackermanFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"ackframe",
		logger,
		300.,
		0,
		testTurnRad,
		0,
		0,
		geometries,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPose(r3.Vector{X: 200, Y: 7000, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90})

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.SquaredNormNoOrientSegmentMetric
	opt.goalMetricConstructor = ik.NewPositionOnlyMetric
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, ok := mp.(*tpSpaceRRTMotionPlanner)
	tp.algOpts.pathdebug = printPath
	if tp.algOpts.pathdebug {
		tp.logger.Debug("$type,X,Y")
		tp.logger.Debugf("$SG,%f,%f\n", 0., 0.)
		tp.logger.Debugf("$SG,%f,%f\n", goalPos.Point().X, goalPos.Point().Y)
	}
	test.That(t, ok, test.ShouldBeTrue)
	plan, err := tp.plan(ctx, goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThanOrEqualTo, 2)

	allPtgs := ackermanFrame.(tpspace.PTGProvider).PTGSolvers()
	lastPose := spatialmath.NewZeroPose()

	if tp.algOpts.pathdebug {
		for _, mynode := range plan {
			trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				if i == 0 {
					tp.logger.Debugf("$WP,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				}
				tp.logger.Debugf("$FINALPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if i == len(trajPts)-1 {
					lastPose = intPose
					break
				}
			}
		}
	}
	tp.planOpts.SmoothIter = 20
	plan = tp.smoothPath(ctx, plan)
	if tp.algOpts.pathdebug {
		lastPose = spatialmath.NewZeroPose()
		for _, mynode := range plan {
			trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				if i == 0 {
					tp.logger.Debugf("$SMOOTHWP,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				}
				tp.logger.Debugf("$SMOOTHPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if pt.Dist >= mynode.Q()[2].Value {
					lastPose = intPose
					break
				}
			}
		}
	}
}

func TestPtgWithObstacle(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}
	ackermanFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"ackframe",
		logger,
		300.,
		0,
		testTurnRad,
		0,
		0,
		geometries,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 6500, Y: 0, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.GoalThreshold = 5
	// obstacles
	obstacle1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{3300, -500, 0}), r3.Vector{180, 1800, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{3300, 1800, 0}), r3.Vector{180, 1800, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle3, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, -1400, 0}), r3.Vector{50000, 30, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle4, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 2400, 0}), r3.Vector{50000, 30, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle5, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{-2500, 0, 0}), r3.Vector{50, 5000, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle6, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{8500, 0, 0}), r3.Vector{50, 5000, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	geoms := []spatialmath.Geometry{obstacle1, obstacle2, obstacle3, obstacle4, obstacle5, obstacle6}

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
	tp.algOpts.pathdebug = printPath
	if tp.algOpts.pathdebug {
		tp.logger.Debug("$type,X,Y")
		for _, geom := range geoms {
			pts := geom.ToPoints(1.)
			for _, pt := range pts {
				if math.Abs(pt.Z) < 0.1 {
					tp.logger.Debugf("$OBS,%f,%f\n", pt.X, pt.Y)
				}
			}
		}
		tp.logger.Debugf("$SG,%f,%f\n", 0., 0.)
		tp.logger.Debugf("$SG,%f,%f\n", goalPos.Point().X, goalPos.Point().Y)
	}
	plan, err := tp.plan(ctx, goalPos, nil)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)

	allPtgs := ackermanFrame.(tpspace.PTGProvider).PTGSolvers()
	lastPose := spatialmath.NewZeroPose()

	if tp.algOpts.pathdebug {
		for _, mynode := range plan {
			trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				if i == 0 {
					tp.logger.Debugf("$WP,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				}
				tp.logger.Debugf("$FINALPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if i == len(trajPts)-1 {
					lastPose = intPose
					break
				}
			}
		}
	}
	tp.planOpts.SmoothIter = 20
	plan = tp.smoothPath(ctx, plan)
	if tp.algOpts.pathdebug {
		lastPose = spatialmath.NewZeroPose()
		for _, mynode := range plan {
			trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				if i == 0 {
					tp.logger.Debugf("$SMOOTHWP,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				}
				tp.logger.Debugf("$SMOOTHPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if pt.Dist >= mynode.Q()[2].Value {
					lastPose = intPose
					break
				}
			}
		}
	}
}

func TestTPsmoothing(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}

	ctx := context.Background()

	ackermanFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"ackframe",
		logger,
		300.,
		0,
		testTurnRad,
		0,
		0,
		geometries,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	// plan which is known to be able to use some smoothing
	planInputs := [][]referenceframe.Input{
		{{0}, {0}, {0}},
		{{3}, {-0.20713797715976653}, {848.2300164692441}},
		{{5}, {0.0314906475636095}, {848.2300108402619}},
		{{5}, {0.0016660735709435135}, {848.2300146893297}},
		{{0}, {0.00021343061342569985}, {408}},
		{{5}, {1.9088870836327245}, {737.7547597081078}},
		{{2}, {-1.3118738553451883}, {848.2300164692441}},
		{{0}, {-3.1070696573964987}, {848.2300164692441}},
		{{0}, {-2.5547017183037877}, {306}},
		{{4}, {-2.31209484211255}, {408}},
		{{0}, {1.1943809502464207}, {571.4368241014894}},
		{{0}, {0.724950779684863}, {848.2300164692441}},
		{{0}, {-1.2295409308605127}, {848.2294213788913}},
		{{5}, {2.677652944060827}, {848.230013198154}},
		{{0}, {2.7618396954635545}, {848.2300164692441}},
		{{0}, {0}, {0}},
	}
	plan := []node{}
	for _, inp := range planInputs {
		thisNode := &basicNode{
			q:    inp,
			cost: inp[2].Value,
		}
		plan = append(plan, thisNode)
	}
	plan, err = rectifyTPspacePath(plan, tp.frame, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)

	// TODO (RSDK-5104) this should be able to be a smaller value once 5104 is complete
	tp.planOpts.SmoothIter = 80

	newplan := tp.smoothPath(ctx, plan)
	test.That(t, newplan, test.ShouldNotBeNil)
	oldcost := 0.
	smoothcost := 0.
	for _, planNode := range plan {
		oldcost += planNode.Cost()
	}
	for _, planNode := range newplan {
		smoothcost += planNode.Cost()
	}
	test.That(t, smoothcost, test.ShouldBeLessThan, oldcost)
}

func TestPtgCheckPlan(t *testing.T) {
	logger := logging.NewTestLogger(t)
	roverGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := []spatialmath.Geometry{roverGeom}
	ackermanFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"ackframe",
		logger,
		300.,
		0,
		testTurnRad,
		0,
		0,
		geometries,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.GoalThreshold = 30.

	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, ackermanFrame.Name(), referenceframe.World, nil)
	test.That(t, err, test.ShouldBeNil)

	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	plan, err := tp.plan(context.Background(), goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	planAsInputs := nodesToInputs(plan)
	test.That(t, err, test.ShouldBeNil)
	steps := []map[string][]referenceframe.Input{}
	for _, resultSlice := range planAsInputs {
		stepMap := sf.sliceToMap(resultSlice)
		steps = append(steps, stepMap)
	}

	startPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})
	errorState := startPose
	inputs := referenceframe.FloatsToInputs([]float64{0, 0, 0})

	t.Run("base case - validate plan without obstacles", func(t *testing.T) {
		collisionPose, err := CheckPlan(ackermanFrame, steps, nil, fs, startPose, inputs, errorState, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisionPose, test.ShouldBeNil)
	})

	t.Run("obstacles blocking path", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2000, 0, 0}), r3.Vector{20, 2000, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		collisionPose, err := CheckPlan(ackermanFrame, steps, worldState, fs, startPose, inputs, errorState, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, collisionPose, test.ShouldBeNil)
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

	t.Run("obstacles NOT in world frame - no collision - integration test", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{25000, -40, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		collisionPose, err := CheckPlan(ackermanFrame, steps, worldState, fs, startPose, inputs, errorState, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisionPose, test.ShouldBeNil)
	})
	t.Run("obstacles NOT in world frame cause collision - integration test", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{2500, 20, 0}),
			r3.Vector{10, 2000, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		collisionPose, err := CheckPlan(ackermanFrame, steps, worldState, fs, startPose, inputs, errorState, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, collisionPose, test.ShouldBeNil)
	})
	t.Run("checking from partial-plan, ensure success with obstacles - integration test", func(t *testing.T) {
		// create obstacle behind where we are
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{0, 20, 0}),
			r3.Vector{10, 200, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		ov := spatialmath.NewOrientationVector().Degrees()
		ov.OZ = 1.0000000000000004
		ov.Theta = -101.42430306111874
		vector := r3.Vector{669.0803080526971, 234.2834571597409, 0}

		startPose := spatialmath.NewPose(vector, ov)

		collisionPose, err := CheckPlan(ackermanFrame, steps[2:len(steps)-1], worldState, fs, startPose, inputs, errorState, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisionPose, test.ShouldBeNil)
	})
	t.Run("verify partial plan with non-nil errorState and obstacle", func(t *testing.T) {
		// create obstacle which is behind where the robot already is, but is on the path it has already traveled
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		errorState := spatialmath.NewPoseFromPoint(r3.Vector{0, 1000, 0})
		startPose = errorState

		collisionPose, err := CheckPlan(ackermanFrame, steps[2:len(steps)-1], worldState, fs, startPose, inputs, errorState, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisionPose, test.ShouldBeNil)
	})
}
