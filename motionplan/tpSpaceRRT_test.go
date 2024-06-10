//go:build !windows

package motionplan

import (
	"bufio"
	"context"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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

	goalPos := spatialmath.NewPose(r3.Vector{X: 200, Y: 7000, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90})

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.StartPose = spatialmath.NewZeroPose()
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, ok := mp.(*tpSpaceRRTMotionPlanner)
	if pathdebug {
		tp.logger.Debug("$type,X,Y")
		tp.logger.Debugf("$SG,%f,%f", 0., 0.)
		tp.logger.Debugf("$SG,%f,%f", goalPos.Point().X, goalPos.Point().Y)
	}
	test.That(t, ok, test.ShouldBeTrue)
	plan, err := tp.plan(ctx, goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThanOrEqualTo, 2)
}

type Rectangle struct {
	StartX float64
	StartY float64
	EndX   float64
	EndY   float64
}

type Circle struct {
	CenterX float64
	CenterY float64
	Radius  float64
}

type Obstacles struct {
	Rectangles []Rectangle
	Circles    []Circle
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

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 3000, Y: 500, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.StartPose = spatialmath.NewPoseFromPoint(r3.Vector{100, 2000, 0})
	opt.GoalThreshold = 5

	// obstacles
	file, _ := os.Open("customObstacles.txt")
	scanner := bufio.NewScanner(file)
	geoms := make([]spatialmath.Geometry, 0)
	blankFound := false
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			blankFound = true
		} else {
			items := strings.Split(line, ":")
			if blankFound { // circles
				centerX, _ := strconv.ParseFloat(items[0], 64)
				centerY, _ := strconv.ParseFloat(items[1], 64)
				radius, _ := strconv.ParseFloat(items[2], 64)
				newObstacle, _ := spatialmath.NewCapsule(spatialmath.NewPoseFromPoint(r3.Vector{centerX, centerY, 0}), radius, 2*radius, "")
				geoms = append(geoms, newObstacle)
			} else {
				startX, _ := strconv.ParseFloat(items[0], 64)
				startY, _ := strconv.ParseFloat(items[1], 64)
				endX, _ := strconv.ParseFloat(items[2], 64)
				endY, _ := strconv.ParseFloat(items[3], 64)
				midX := (startX + endX) / 2
				midY := (startY + endY) / 2
				width := endX - startX
				height := endY - startY
				newObstacle, _ := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{midX, midY, 0}), r3.Vector{width, height, 1}, "")
				geoms = append(geoms, newObstacle)
			}
		}

	}
	// obstacle1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{217, 150, 0}), r3.Vector{424, 289, 1}, "")
	// test.That(t, err, test.ShouldBeNil)
	// obstacle2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{601, 482, 0}), r3.Vector{377, 234, 1}, "")
	// test.That(t, err, test.ShouldBeNil)
	// // // obstacle3, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, -1400, 0}), r3.Vector{50000, 30, 1}, "")
	// // // test.That(t, err, test.ShouldBeNil)
	// // // obstacle4, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 2400, 0}), r3.Vector{50000, 30, 1}, "")
	// // // test.That(t, err, test.ShouldBeNil)
	// // // obstacle5, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{-2500, 0, 0}), r3.Vector{50, 5000, 1}, "")
	// // // test.That(t, err, test.ShouldBeNil)
	// // // obstacle6, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{8500, 0, 0}), r3.Vector{50, 5000, 1}, "")
	// // // test.That(t, err, test.ShouldBeNil)

	// geoms := []spatialmath.Geometry{obstacle1, obstacle2}

	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, ackermanFrame.Name(), referenceframe.World, nil)
	test.That(t, err, test.ShouldBeNil)

	seedMap := referenceframe.StartPositions(fs)
	frameInputs, err := sf.mapToSlice(seedMap)
	test.That(t, err, test.ShouldBeNil)

	// create robot collision entities
	movingGeometriesInFrame, err := sf.Geometries(frameInputs)
	movingRobotGeometries := movingGeometriesInFrame.Geometries() // solver frame returns geoms in frame World
	test.That(t, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatialmath.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
	test.That(t, err, test.ShouldBeNil)
	for name, geometries := range frameSystemGeometries {
		if !sf.movingFrame(name) {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, seedMap)
	test.That(t, err, test.ShouldBeNil)

	collisionConstraints, err := createAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil,
		defaultCollisionBufferMM,
	)

	test.That(t, err, test.ShouldBeNil)

	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(time.Now().Unix())), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)
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
		tp.logger.Debugf("$SG,%f,%f", opt.StartPose.Point().X, opt.StartPose.Point().Y)
		tp.logger.Debugf("$SG,%f,%f", goalPos.Point().X, goalPos.Point().Y)
	}
	planStart := time.Now()
	plan, err := tp.plan(ctx, goalPos, nil)
	planEnd := time.Now()

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)

	tp.planOpts.SmoothIter = 20
	smoothStart := time.Now()
	newplan := tp.smoothPath(ctx, plan)
	smoothEnd := time.Now()
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
	allPtgs := tp.tpFrame.PTGSolvers()

	tp.logger.Debugf("------------ OLD PLAN ALPHA-DEEZ NUTS ---------------\n")
	oldPlanCurvature := 0.
	for _, mynode := range plan {
		trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(
			mynode.Q()[1].Value,
			mynode.Q()[2].Value,
			mynode.Q()[3].Value,
			tp.planOpts.Resolution,
		)
		curv, _ := allPtgs[int(mynode.Q()[0].Value)].Curvature(mynode.Q()[1].Value)
		oldPlanCurvature = oldPlanCurvature + curv
		for i, pt := range trajPts {
			tp.logger.Debugf("i: %v | Alpha: %v | D: %v | PTG: %v \n", i, pt.Alpha, pt.Dist, mynode.Q()[0].Value)
		}
	}
	tp.logger.Debugf("---------------- NEW PLAN ALPHA-DEEZ NUTS ---------------- \n")
	newPlanCurvature := 0.
	for _, mynode := range newplan {
		trajPts, _ := allPtgs[int(mynode.Q()[0].Value)].Trajectory(
			mynode.Q()[1].Value,
			mynode.Q()[2].Value,
			mynode.Q()[3].Value,
			tp.planOpts.Resolution,
		)
		curv, _ := allPtgs[int(mynode.Q()[0].Value)].Curvature(mynode.Q()[1].Value)
		newPlanCurvature = newPlanCurvature + curv
		for i, pt := range trajPts {
			tp.logger.Debugf("i: %v | Alpha: %v | D: %v \n", i, pt.Alpha, pt.Dist)
		}
	}
	for i, ptg := range allPtgs {
		tp.logger.Debugf("i: %v | ptgType: %T\n", i, ptg)
	}
	tp.logger.Debugf("planTime: %v\n", planEnd.Sub(planStart))
	tp.logger.Debugf("smoothTime: %v\n", smoothEnd.Sub(smoothStart))
	tp.logger.Debugf("Old Cost: %v | Old Curvature Cost: %v\n", oldcost, oldPlanCurvature)
	tp.logger.Debugf("Smooth Cost: %v | Smooth Curvature Cost: %v\n", smoothcost, newPlanCurvature)
	tp.logger.Debugf("path len: %v\n", len(newplan))

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

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
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
			q:    inp,
			cost: inp[3].Value - inp[2].Value,
		}
		plan = append(plan, thisNode)
	}
	plan, err = rectifyTPspacePath(plan, tp.frame, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)

	tp.planOpts.SmoothIter = 75

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
		testTurnRad,
		0,
		geometries,
		false,
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions(ackermanFrame)
	opt.StartPose = spatialmath.NewZeroPose()
	opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(30.)
	opt.GoalThreshold = 30.

	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, ackermanFrame.Name(), referenceframe.World, nil)
	test.That(t, err, test.ShouldBeNil)

	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	nodes, err := tp.plan(context.Background(), goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	plan, err := newRRTPlan(nodes, sf, true)
	test.That(t, err, test.ShouldBeNil)

	startPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})
	inputs := plan.Trajectory()[0]

	t.Run("base case - validate plan without obstacles", func(t *testing.T) {
		executionState := ExecutionState{
			plan:          plan,
			index:         0,
			currentInputs: inputs,
			currentPose: map[string]*referenceframe.PoseInFrame{
				ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
			},
		}
		err = CheckPlan(ackermanFrame, executionState, nil, fs, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("obstacles blocking path", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2000, 0, 0}), r3.Vector{20, 2000, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		executionState := ExecutionState{
			plan:          plan,
			index:         0,
			currentInputs: inputs,
			currentPose: map[string]*referenceframe.PoseInFrame{
				ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
			},
		}
		err = CheckPlan(ackermanFrame, executionState, worldState, fs, math.Inf(1), logger)
		test.That(t, err, test.ShouldNotBeNil)
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
		executionState := ExecutionState{
			plan:          plan,
			index:         1,
			currentInputs: inputs,
			currentPose: map[string]*referenceframe.PoseInFrame{
				ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
			},
		}
		err = CheckPlan(ackermanFrame, executionState, worldState, fs, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
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

		executionState := ExecutionState{
			plan:          plan,
			index:         1,
			currentInputs: inputs,
			currentPose: map[string]*referenceframe.PoseInFrame{
				ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
			},
		}
		err = CheckPlan(ackermanFrame, executionState, worldState, fs, math.Inf(1), logger)
		test.That(t, err, test.ShouldNotBeNil)
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

		executionState := ExecutionState{
			plan:          plan,
			index:         2,
			currentInputs: inputs,
			currentPose: map[string]*referenceframe.PoseInFrame{
				ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
			},
		}
		err = CheckPlan(ackermanFrame, executionState, worldState, fs, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("verify partial plan with non-nil errorState and obstacle", func(t *testing.T) {
		// create obstacle which is behind where the robot already is, but is on the path it has already traveled
		box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 1}, "obstacle")
		test.That(t, err, test.ShouldBeNil)
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{box})}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		remainingPlan, err := RemainingPlan(plan, 2)
		test.That(t, err, test.ShouldBeNil)

		pathPose := remainingPlan.Path()[0][ackermanFrame.Name()].Pose()
		startPose := spatialmath.NewPose(r3.Vector{0, 1000, 0}, pathPose.Orientation())

		executionState := ExecutionState{
			plan:          plan,
			index:         2,
			currentInputs: inputs,
			currentPose: map[string]*referenceframe.PoseInFrame{
				ackermanFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose),
			},
		}
		err = CheckPlan(ackermanFrame, executionState, worldState, fs, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})
}

func planToTpspaceRec(plan Plan, f referenceframe.Frame) ([]node, error) {
	nodes := []node{}
	for _, inp := range plan.Trajectory() {
		thisNode := &basicNode{
			q:    inp[f.Name()],
			cost: inp[f.Name()][3].Value,
		}
		nodes = append(nodes, thisNode)
	}
	return rectifyTPspacePath(nodes, f, spatialmath.NewZeroPose())
}
