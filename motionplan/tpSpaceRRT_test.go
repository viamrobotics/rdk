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
		testTurnRad,
		0,
		geometries,
		false,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	goalPos := spatialmath.NewPose(r3.Vector{X: 200, Y: 7000, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90})
	goal := &PlanState{poses: referenceframe.FrameSystemPoses{
		ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, goalPos),
	}}
	start := &PlanState{poses: referenceframe.FrameSystemPoses{
		ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewZeroPose()),
	}}

	opt := newBasicPlannerOptions()
	opt.poseDistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.scoreFunc = tpspace.NewPTGDistanceMetric([]string{ackermanFrame.Name()})
	opt.PlannerConstructor = newTPSpaceMotionPlanner

	opt.fillMotionChains(fs, goal)

	mp, err := newTPSpaceMotionPlanner(fs, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, ok := mp.(*tpSpaceRRTMotionPlanner)
	test.That(t, ok, test.ShouldBeTrue)

	if pathdebug {
		tp.logger.Debug("$type,X,Y")
		tp.logger.Debugf("$SG,%f,%f", 0., 0.)
		tp.logger.Debugf("$SG,%f,%f", goalPos.Point().X, goalPos.Point().Y)
	}

	plan, err := tp.plan(ctx, start, goal)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThanOrEqualTo, 2)
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
		testTurnRad,
		0,
		geometries,
		false,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	// Create frame system and add ackerman frame
	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	// Set up start and goal poses
	startPose := spatialmath.NewPoseFromPoint(r3.Vector{0, -1000, 0})
	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 6500, Y: 0, Z: 0})

	start := &PlanState{poses: map[string]*referenceframe.PoseInFrame{
		ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
	}}
	goal := &PlanState{poses: map[string]*referenceframe.PoseInFrame{
		ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, goalPos),
	}}

	// Create obstacles
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

	// Set up world state
	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	// Initialize planner options
	opt := newBasicPlannerOptions()
	opt.poseDistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.GoalThreshold = 5
	opt.PlannerConstructor = newTPSpaceMotionPlanner
	opt.scoreFunc = tpspace.NewPTGDistanceMetric([]string{ackermanFrame.Name()})
	opt.fillMotionChains(fs, goal)

	// Create collision constraints
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, nil)
	test.That(t, err, test.ShouldBeNil)

	_, collisionConstraints, err := createAllCollisionConstraints(
		geometries, // moving geometries
		nil,        // static robot geometries
		worldGeometries.Geometries(),
		nil, nil,
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)

	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	// Create and initialize planner
	mp, err := newTPSpaceMotionPlanner(fs, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	// Debug logging if enabled
	if pathdebug {
		tp.logger.Debug("$type,X,Y")
		for _, geom := range geoms {
			pts := geom.ToPoints(1.)
			for _, pt := range pts {
				if math.Abs(pt.Z) < 0.1 {
					tp.logger.Debugf("$OBS,%f,%f", pt.X, pt.Y)
				}
			}
		}
		tp.logger.Debugf("$SG,%f,%f", startPose.Point().X, startPose.Point().Y)
		tp.logger.Debugf("$SG,%f,%f", goalPos.Point().X, goalPos.Point().Y)
	}

	// Plan and verify results
	plan, err := tp.plan(ctx, start, goal)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)

	// Test smoothing
	tp.planOpts.SmoothIter = 80
	newplan := tp.smoothPath(ctx, plan)
	test.That(t, newplan, test.ShouldNotBeNil)

	// Compare costs
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
		testTurnRad,
		0,
		geometries,
		false,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	// Create frame system and add ackerman frame
	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	// Initialize planner options
	opt := newBasicPlannerOptions()
	opt.poseDistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.scoreFunc = tpspace.NewPTGDistanceMetric([]string{ackermanFrame.Name()})
	opt.PlannerConstructor = newTPSpaceMotionPlanner

	// Needed to determine motion chains
	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 6500, Y: 0, Z: 0})
	goal := &PlanState{poses: map[string]*referenceframe.PoseInFrame{
		ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, goalPos),
	}}

	opt.fillMotionChains(fs, goal)

	// Create and initialize planner
	mp, err := newTPSpaceMotionPlanner(fs, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	// plan which is known to be able to use some smoothing
	planInputs := [][]referenceframe.Input{
		{{0}, {0}, {0}, {0}},
		{{3}, {-0.20713797715976653}, {0}, {848.2300164692441}},
		{{4}, {0.0314906475636095}, {0}, {848.2300108402619}},
		{{4}, {0.0016660735709435135}, {0}, {848.2300146893297}},
		{{0}, {0.00021343061342569985}, {0}, {408}},
		{{4}, {1.9088870836327245}, {0}, {737.7547597081078}},
		{{2}, {-1.3118738553451883}, {0}, {848.2300164692441}},
		{{0}, {-3.1070696573964987}, {0}, {848.2300164692441}},
		{{0}, {-2.5547017183037877}, {0}, {306}},
		{{4}, {-2.31209484211255}, {0}, {408}},
		{{0}, {1.1943809502464207}, {0}, {571.4368241014894}},
		{{0}, {0.724950779684863}, {0}, {848.2300164692441}},
		{{0}, {-1.2295409308605127}, {0}, {848.2294213788913}},
		{{4}, {2.677652944060827}, {0}, {848.230013198154}},
		{{0}, {2.7618396954635545}, {0}, {848.2300164692441}},
		{{0}, {0}, {0}, {0}},
	}
	plan := []node{}
	for _, inp := range planInputs {
		thisNode := &basicNode{
			q:    referenceframe.FrameSystemInputs{ackermanFrame.Name(): inp},
			cost: inp[3].Value - inp[2].Value,
		}
		plan = append(plan, thisNode)
	}
	plan, err = rectifyTPspacePath(plan, tp.tpFrame, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)

	tp.planOpts.SmoothIter = 20

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

func planToTpspaceRec(plan Plan, f referenceframe.Frame) ([]node, error) {
	nodes := []node{}
	for _, inp := range plan.Trajectory() {
		thisNode := &basicNode{
			q:    inp,
			cost: math.Abs(inp[f.Name()][3].Value - inp[f.Name()][2].Value),
		}
		nodes = append(nodes, thisNode)
	}
	return rectifyTPspacePath(nodes, f, spatialmath.NewZeroPose())
}
