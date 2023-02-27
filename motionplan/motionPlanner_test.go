package motionplan

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.uber.org/zap"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	home6 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
)

var logger, _ = zap.Config{
	Level:             zap.NewAtomicLevelAt(zap.FatalLevel),
	Encoding:          "console",
	DisableStacktrace: true,
}.Build()

type planConfig struct {
	Start      []frame.Input
	Goal       spatialmath.Pose
	RobotFrame frame.Frame
	Options    *plannerOptions
}

type planConfigConstructor func() (*planConfig, error)

func TestUnconstrainedMotion(t *testing.T) {
	t.Parallel()
	planners := []plannerConstructor{
		newRRTStarConnectMotionPlanner,
		newCBiRRTMotionPlanner,
	}
	testCases := []struct {
		name   string
		config planConfigConstructor
	}{
		{"2D plan test", simple2DMap},
		{"6D plan test", simpleUR5eMotion},
		{"7D plan test", simpleXArmMotion},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			for _, p := range planners {
				testPlanner(t, p, testCase.config, 1)
			}
		})
	}
}

func TestConstrainedMotion(t *testing.T) {
	t.Parallel()
	planners := []plannerConstructor{
		newCBiRRTMotionPlanner,
	}
	testCases := []struct {
		name   string
		config planConfigConstructor
	}{
		{"linear motion, no-spill", constrainedXArmMotion},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			for _, p := range planners {
				testPlanner(t, p, testCase.config, 1)
			}
		})
	}
}

// TestConstrainedArmMotion tests a simple linear motion on a longer path, with a no-spill constraint.
func constrainedXArmMotion() (*planConfig, error) {
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm7_kinematics.json"), "")
	if err != nil {
		return nil, err
	}

	// Test ability to arrive at another position
	pos := spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: -206, Y: 100, Z: 120, OZ: -1})

	opt := newBasicPlannerOptions()
	orientMetric := NewPoseFlexOVMetric(pos, 0.09)

	oFunc := orientDistToRegion(pos.Orientation(), 0.1)
	oFuncMet := func(from, to spatialmath.Pose) float64 {
		return oFunc(from.Orientation())
	}
	orientConstraint := func(o spatialmath.Orientation) bool {
		return oFunc(o) == 0
	}

	opt.SetMetric(orientMetric)
	opt.SetPathDist(oFuncMet)
	opt.AddConstraint("orientation", NewOrientationConstraint(orientConstraint))

	return &planConfig{
		Start:      home7,
		Goal:       pos,
		RobotFrame: model,
		Options:    opt,
	}, nil
}

func TestPlanningWithGripper(t *testing.T) {
	fs := frame.NewEmptySimpleFrameSystem("")
	ur5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "ur")
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(ur5e, fs.World())
	test.That(t, err, test.ShouldBeNil)
	bc, _ := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 75}), r3.Vector{200, 200, 200}, "")
	gripper, err := frame.NewStaticFrameWithGeometry("gripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 150}), bc)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gripper, ur5e)
	test.That(t, err, test.ShouldBeNil)
	zeroPos := frame.StartPositions(fs)

	newPose := frame.NewPoseInFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{100, 100, 0}))
	solutionMap, err := PlanMotion(
		context.Background(),
		logger.Sugar(),
		newPose,
		gripper,
		zeroPos,
		fs,
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(solutionMap), test.ShouldBeGreaterThanOrEqualTo, 2)
}

// simple2DMapConfig returns a planConfig with the following map
//   - start at (-9, 9) and end at (9, 9)
//   - bounds are from (-10, -10) to (10, 10)
//   - obstacle from (-4, 2) to (4, 10)
//
// ------------------------
// | +      |    |      + |
// |        |    |        |
// |        |    |        |
// |        |    |        |
// |        ------        |
// |          *           |
// |                      |
// |                      |
// |                      |
// ------------------------.
func simple2DMap() (*planConfig, error) {
	// build model
	limits := []frame.Limit{{Min: -100, Max: 100}, {Min: -100, Max: 100}}
	physicalGeometry, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, "")
	if err != nil {
		return nil, err
	}
	modelName := "mobile-base"
	model, err := frame.NewMobile2DFrame(modelName, limits, physicalGeometry)
	if err != nil {
		return nil, err
	}

	// add it to the frame system
	fs := frame.NewEmptySimpleFrameSystem("test")
	if err := fs.AddFrame(model, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	// obstacles
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{0, 50, 0}), r3.Vector{80, 80, 1}, "")
	if err != nil {
		return nil, err
	}
	worldState := &frame.WorldState{
		Obstacles: []*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, map[string]spatialmath.Geometry{"b": box})},
	}

	// setup planner options
	opt := newBasicPlannerOptions()
	startInput := frame.StartPositions(fs)
	startInput[modelName] = frame.FloatsToInputs([]float64{-90., 90.})
	collisionConstraint, err := newObstacleConstraint(model, fs, worldState, startInput, nil, false)
	if err != nil {
		return nil, err
	}
	opt.AddConstraint("collision", collisionConstraint)

	return &planConfig{
		Start:      startInput[modelName],
		Goal:       spatialmath.NewPoseFromPoint(r3.Vector{X: 90, Y: 90, Z: 0}),
		RobotFrame: model,
		Options:    opt,
	}, nil
}

// simpleArmMotion tests moving an xArm7.
func simpleXArmMotion() (*planConfig, error) {
	xarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm7_kinematics.json"), "")
	if err != nil {
		return nil, err
	}

	// add it to the frame system
	fs := frame.NewEmptySimpleFrameSystem("test")
	if err := fs.AddFrame(xarm, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	// setup planner options
	opt := newBasicPlannerOptions()
	collisionConstraint, err := newSelfCollisionConstraint(xarm, frame.StartPositions(fs), nil, false)
	if err != nil {
		return nil, err
	}
	opt.AddConstraint("collision", collisionConstraint)

	return &planConfig{
		Start:      home7,
		Goal:       spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: 206, Y: 100, Z: 120, OZ: -1}),
		RobotFrame: xarm,
		Options:    opt,
	}, nil
}

// simpleUR5eMotion tests a simple motion for a UR5e.
func simpleUR5eMotion() (*planConfig, error) {
	ur5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
	if err != nil {
		return nil, err
	}
	fs := frame.NewEmptySimpleFrameSystem("test")
	if err = fs.AddFrame(ur5e, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	// setup planner options
	opt := newBasicPlannerOptions()
	collisionConstraint, err := newSelfCollisionConstraint(ur5e, frame.StartPositions(fs), nil, false)
	if err != nil {
		return nil, err
	}
	opt.AddConstraint("collision", collisionConstraint)

	return &planConfig{
		Start:      home6,
		Goal:       spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: -750, Y: -250, Z: 200, OX: -1}),
		RobotFrame: ur5e,
		Options:    opt,
	}, nil
}

// testPlanner is a helper function that takes a planner and a planning query specified through a config object and tests that it
// returns a valid set of waypoints.
func testPlanner(t *testing.T, plannerFunc plannerConstructor, config planConfigConstructor, seed int) {
	t.Helper()

	// plan
	cfg, err := config()
	test.That(t, err, test.ShouldBeNil)
	mp, err := plannerFunc(cfg.RobotFrame, rand.New(rand.NewSource(int64(seed))), logger.Sugar(), cfg.Options)
	test.That(t, err, test.ShouldBeNil)
	path, err := mp.plan(context.Background(), cfg.Goal, cfg.Start)
	test.That(t, err, test.ShouldBeNil)
	// test that path doesn't violate constraints
	test.That(t, len(path), test.ShouldBeGreaterThanOrEqualTo, 2)
	for j := 0; j < len(path)-1; j++ {
		ok, _ := cfg.Options.constraintHandler.CheckConstraintPath(&ConstraintInput{
			StartInput: path[j],
			EndInput:   path[j+1],
			Frame:      cfg.RobotFrame,
		}, cfg.Options.Resolution)
		test.That(t, ok, test.ShouldBeTrue)
	}
}

func makeTestFS(t *testing.T) frame.FrameSystem {
	t.Helper()
	fs := frame.NewEmptySimpleFrameSystem("test")

	urOffset, err := frame.NewStaticFrame("urOffset", spatialmath.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urOffset, fs.World())
	gantryOffset, err := frame.NewStaticFrame("gantryOffset", spatialmath.NewPoseFromPoint(r3.Vector{-50, -50, -200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	gantryX, err := frame.NewTranslationalFrame("gantryX", r3.Vector{1, 0, 0}, frame.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryX, gantryOffset)
	gantryY, err := frame.NewTranslationalFrame("gantryY", r3.Vector{0, 1, 0}, frame.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryY, gantryX)

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantryY)

	modelUR5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelUR5e, urOffset)

	// Note that positive Z is always "forwards". If the position of the arm is such that it is pointing elsewhere,
	// the resulting translation will be similarly oriented
	urCamera, err := frame.NewStaticFrame("urCamera", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 30}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urCamera, modelUR5e)

	// Add static frame for the gripper
	bc, _ := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 100}), r3.Vector{200, 200, 200}, "")
	xArmVgripper, err := frame.NewStaticFrameWithGeometry("xArmVgripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 200}), bc)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(xArmVgripper, modelXarm)

	return fs
}

func TestArmOOBSolve(t *testing.T) {
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)

	// Set a goal unreachable by the UR due to sheer distance
	goal1 := spatialmath.NewPose(r3.Vector{X: 257, Y: 21000, Z: -300}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	_, err := PlanMotion(
		context.Background(),
		logger.Sugar(),
		frame.NewPoseInFrame(frame.World, goal1),
		fs.Frame("urCamera"),
		positions,
		fs,
		nil,
		nil,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, errIKSolve.Error())
}

func TestArmObstacleSolve(t *testing.T) {
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)

	// Set an obstacle such that it is impossible to reach the goal without colliding with it
	obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: 257, Y: 210, Z: -300}), r3.Vector{10, 10, 100}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := make(map[string]spatialmath.Geometry)
	geometries["obstacle"] = obstacle
	obstacles := frame.NewGeometriesInFrame(frame.World, geometries)
	worldState := &frame.WorldState{Obstacles: []*frame.GeometriesInFrame{obstacles}}

	// Set a goal unreachable by the UR
	goal1 := spatialmath.NewPose(r3.Vector{X: 257, Y: 210, Z: -300}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	_, err = PlanMotion(
		context.Background(),
		logger.Sugar(),
		frame.NewPoseInFrame(frame.World, goal1),
		fs.Frame("urCamera"),
		positions,
		fs,
		worldState,
		nil,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(
		t,
		err.Error(),
		test.ShouldEqual,
		"all IK solutions failed constraints. Failures: { "+defaultObstacleConstraintName+": 100.00% }, ",
	)
}

func TestArmAndGantrySolve(t *testing.T) {
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)
	pointXarmGripper := spatialmath.NewPoseFromPoint(r3.Vector{157., -50, -288})
	transformPoint, err := fs.Transform(positions, frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincident(transformPoint.(*frame.PoseInFrame).Pose(), pointXarmGripper), test.ShouldBeTrue)

	// Set a goal such that the gantry and arm must both be used to solve
	goal1 := spatialmath.NewPose(r3.Vector{X: 257, Y: 2100, Z: -300}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	newPos, err := PlanMotion(
		context.Background(),
		logger.Sugar(),
		frame.NewPoseInFrame(frame.World, goal1),
		fs.Frame("xArmVgripper"),
		positions,
		fs,
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	solvedPose, err := fs.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal1, 0.01), test.ShouldBeTrue)
}

func TestMultiArmSolve(t *testing.T) {
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)
	// Solve such that the ur5 and xArm are pointing at each other, 60mm from gripper to camera
	goal2 := spatialmath.NewPose(r3.Vector{Z: 60}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	newPos, err := PlanMotion(
		context.Background(),
		logger.Sugar(),
		frame.NewPoseInFrame("urCamera", goal2),
		fs.Frame("xArmVgripper"),
		positions,
		fs,
		nil,
		map[string]interface{}{"max_ik_solutions": 100, "timeout": 150.0},
	)
	test.That(t, err, test.ShouldBeNil)

	// Both frames should wind up at the goal relative to one another
	solvedPose, err := fs.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()), "urCamera")
	test.That(t, err, test.ShouldBeNil)
	solvedPose2, err := fs.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("urCamera", spatialmath.NewZeroPose()), "xArmVgripper")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal2, 0.1), test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose2.(*frame.PoseInFrame).Pose(), goal2, 0.1), test.ShouldBeTrue)
}

func TestSliceUniq(t *testing.T) {
	fs := makeTestFS(t)
	slice := []frame.Frame{}
	slice = append(slice, fs.Frame("urCamera"))
	slice = append(slice, fs.Frame("gantryOffset"))
	slice = append(slice, fs.Frame("xArmVgripper"))
	slice = append(slice, fs.Frame("urCamera"))
	uniqd := uniqInPlaceSlice(slice)
	test.That(t, len(uniqd), test.ShouldEqual, 3)
}

func TestSolverFrameGeometries(t *testing.T) {
	fs := makeTestFS(t)
	sFrames, err := fs.TracebackFrame(fs.Frame("xArmVgripper"))
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, sFrames, frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)

	sfPlanner, err := newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	position, err := sfPlanner.PlanSingleWaypoint(
		context.Background(),
		sf.sliceToMap(make([]frame.Input, len(sf.DoF()))),
		spatialmath.NewPoseFromPoint(r3.Vector{300, 300, 100}),
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	gf, _ := sf.Geometries(position[len(position)-1])
	test.That(t, gf, test.ShouldNotBeNil)
	gripperCenter := gf.Geometries()["xArmVgripper"].Pose().Point()
	test.That(t, spatialmath.R3VectorAlmostEqual(gripperCenter, r3.Vector{300, 300, 0}, 1e-2), test.ShouldBeTrue)
}

func TestMovementWithGripper(t *testing.T) {
	// TODO(rb): move these tests to a separate repo eventually, as they take up too much time for general CI pipeline
	t.Skip()

	// setup solverFrame and planning query
	fs := makeTestFS(t)
	fs.RemoveFrame(fs.Frame("urOffset"))
	sFrames, err := fs.TracebackFrame(fs.Frame("xArmVgripper"))
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, sFrames, frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	goal := spatialmath.NewPose(r3.Vector{500, 0, -300}, &spatialmath.OrientationVector{OZ: -1})
	zeroPosition := sf.sliceToMap(make([]frame.Input, len(sf.DoF())))

	// linearly plan with the gripper
	motionConfig := make(map[string]interface{})
	motionConfig["motion_profile"] = LinearMotionProfile
	sfPlanner, err := newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err := sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, nil, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// plan around the obstacle with the gripper
	obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{300, 0, -400}), r3.Vector{50, 500, 500}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := make(map[string]spatialmath.Geometry)
	geometries["obstacle"] = obstacle
	obstacles := frame.NewGeometriesInFrame(frame.World, geometries)
	worldState := &frame.WorldState{Obstacles: []*frame.GeometriesInFrame{obstacles}}
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// plan with end of arm with gripper attached - this will fail
	sFrames, err = fs.TracebackFrame(fs.Frame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	sf, err = newSolverFrame(fs, sFrames, frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	goal = spatialmath.NewPose(r3.Vector{500, 0, -100}, &spatialmath.OrientationVector{OZ: -1})
	zeroPosition = sf.sliceToMap(make([]frame.Input, len(sf.DoF())))
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	_, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, motionConfig)
	test.That(t, err, test.ShouldNotBeNil)

	// remove linear constraint and try again
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// remove gripper and try with linear constraint
	fs.RemoveFrame(fs.Frame("xArmVgripper"))
	sFrames, err = fs.TracebackFrame(fs.Frame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	sf, err = newSolverFrame(fs, sFrames, frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	zeroPosition = sf.sliceToMap(make([]frame.Input, len(sf.DoF())))
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)
}
