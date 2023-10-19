package motionplan

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	commonpb "go.viam.com/api/common/v1"
	motionpb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
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
		tcCopy := testCase
		t.Run(tcCopy.name, func(t *testing.T) {
			t.Parallel()
			for _, p := range planners {
				testPlanner(t, p, tcCopy.config, 1)
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
		tcCopy := testCase
		t.Run(tcCopy.name, func(t *testing.T) {
			t.Parallel()
			for _, p := range planners {
				testPlanner(t, p, tcCopy.config, 1)
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

	opt := newBasicPlannerOptions(model)
	opt.SmoothIter = 2
	orientMetric := ik.NewPoseFlexOVMetricConstructor(0.09)

	oFunc := ik.OrientDistToRegion(pos.Orientation(), 0.1)
	oFuncMet := func(from *ik.State) float64 {
		err := resolveStatesToPositions(from)
		if err != nil {
			return math.Inf(1)
		}
		return oFunc(from.Position.Orientation())
	}
	orientConstraint := func(cInput *ik.State) bool {
		err := resolveStatesToPositions(cInput)
		if err != nil {
			return false
		}

		return oFunc(cInput.Position.Orientation()) == 0
	}

	opt.goalMetricConstructor = orientMetric
	opt.SetPathMetric(oFuncMet)
	opt.AddStateConstraint("orientation", orientConstraint)

	return &planConfig{
		Start:      home7,
		Goal:       pos,
		RobotFrame: model,
		Options:    opt,
	}, nil
}

func TestPlanningWithGripper(t *testing.T) {
	fs := frame.NewEmptyFrameSystem("")
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
	solutionMap, err := PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               newPose,
		Frame:              gripper,
		StartConfiguration: zeroPos,
		FrameSystem:        fs,
	})
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
	limits := []frame.Limit{{Min: -100, Max: 100}, {Min: -100, Max: 100}, {Min: -2 * math.Pi, Max: 2 * math.Pi}}
	physicalGeometry, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, "")
	if err != nil {
		return nil, err
	}
	modelName := "mobile-base"
	model, err := frame.New2DMobileModelFrame(modelName, limits, physicalGeometry)
	if err != nil {
		return nil, err
	}

	// add it to the frame system
	fs := frame.NewEmptyFrameSystem("test")
	if err := fs.AddFrame(model, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	// obstacles
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{0, 50, 0}), r3.Vector{80, 80, 1}, "")
	if err != nil {
		return nil, err
	}
	worldState, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{box})},
		nil,
	)
	if err != nil {
		return nil, err
	}

	// setup planner options
	opt := newBasicPlannerOptions(model)
	startInput := frame.StartPositions(fs)
	startInput[modelName] = frame.FloatsToInputs([]float64{-90., 90., 0})
	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 90, Y: 90, Z: 0})
	opt.SetGoal(goal)
	sf, err := newSolverFrame(fs, modelName, frame.World, startInput)
	if err != nil {
		return nil, err
	}
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, worldState, frame.StartPositions(fs), nil)
	if err != nil {
		return nil, err
	}
	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	return &planConfig{
		Start:      startInput[modelName],
		Goal:       goal,
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
	fs := frame.NewEmptyFrameSystem("test")
	if err := fs.AddFrame(xarm, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	goal := spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: 206, Y: 100, Z: 120, OZ: -1})

	// setup planner options
	opt := newBasicPlannerOptions(xarm)
	opt.SmoothIter = 20
	opt.SetGoal(goal)
	sf, err := newSolverFrame(fs, xarm.Name(), frame.World, frame.StartPositions(fs))
	if err != nil {
		return nil, err
	}
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, nil, frame.StartPositions(fs), nil)
	if err != nil {
		return nil, err
	}
	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	return &planConfig{
		Start:      home7,
		Goal:       goal,
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
	fs := frame.NewEmptyFrameSystem("test")
	if err = fs.AddFrame(ur5e, fs.Frame(frame.World)); err != nil {
		return nil, err
	}
	goal := spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: -750, Y: -250, Z: 200, OX: -1})

	// setup planner options
	opt := newBasicPlannerOptions(ur5e)
	opt.SmoothIter = 20
	opt.SetGoal(goal)
	sf, err := newSolverFrame(fs, ur5e.Name(), frame.World, frame.StartPositions(fs))
	if err != nil {
		return nil, err
	}
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, nil, frame.StartPositions(fs), nil)
	if err != nil {
		return nil, err
	}
	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	return &planConfig{
		Start:      home6,
		Goal:       goal,
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
	pathNodes, err := mp.plan(context.Background(), cfg.Goal, cfg.Start)
	test.That(t, err, test.ShouldBeNil)
	path := nodesToInputs(pathNodes)

	// test that path doesn't violate constraints
	test.That(t, len(path), test.ShouldBeGreaterThanOrEqualTo, 2)
	for j := 0; j < len(path)-1; j++ {
		ok, _ := cfg.Options.ConstraintHandler.CheckSegmentAndStateValidity(&ik.Segment{
			StartConfiguration: path[j],
			EndConfiguration:   path[j+1],
			Frame:              cfg.RobotFrame,
		}, cfg.Options.Resolution)
		test.That(t, ok, test.ShouldBeTrue)
	}
}

func makeTestFS(t *testing.T) frame.FrameSystem {
	t.Helper()
	fs := frame.NewEmptyFrameSystem("test")

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
	_, err := PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               frame.NewPoseInFrame(frame.World, goal1),
		Frame:              fs.Frame("urCamera"),
		StartConfiguration: positions,
		FrameSystem:        fs,
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, errIKSolve.Error())
}

func TestArmObstacleSolve(t *testing.T) {
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)

	// Set an obstacle such that it is impossible to reach the goal without colliding with it
	obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: 257, Y: 210, Z: -300}), r3.Vector{10, 10, 100}, "")
	test.That(t, err, test.ShouldBeNil)
	worldState, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{obstacle})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	// Set a goal unreachable by the UR
	goal1 := spatialmath.NewPose(r3.Vector{X: 257, Y: 210, Z: -300}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	_, err = PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               frame.NewPoseInFrame(frame.World, goal1),
		Frame:              fs.Frame("urCamera"),
		StartConfiguration: positions,
		FrameSystem:        fs,
		WorldState:         worldState,
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errIKConstraint)
}

func TestArmAndGantrySolve(t *testing.T) {
	t.Parallel()
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)
	pointXarmGripper := spatialmath.NewPoseFromPoint(r3.Vector{157., -50, -288})
	transformPoint, err := fs.Transform(
		positions,
		frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()),
		frame.World,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincident(transformPoint.(*frame.PoseInFrame).Pose(), pointXarmGripper), test.ShouldBeTrue)

	// Set a goal such that the gantry and arm must both be used to solve
	goal1 := spatialmath.NewPose(r3.Vector{X: 257, Y: 2100, Z: -300}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	plan, err := PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               frame.NewPoseInFrame(frame.World, goal1),
		Frame:              fs.Frame("xArmVgripper"),
		StartConfiguration: positions,
		FrameSystem:        fs,
		Options:            map[string]interface{}{"smooth_iter": 5},
	})
	test.That(t, err, test.ShouldBeNil)
	solvedPose, err := fs.Transform(
		plan[len(plan)-1],
		frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()),
		frame.World,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal1, 0.01), test.ShouldBeTrue)
}

func TestMultiArmSolve(t *testing.T) {
	t.Parallel()
	fs := makeTestFS(t)
	positions := frame.StartPositions(fs)
	// Solve such that the ur5 and xArm are pointing at each other, 40mm from gripper to camera
	goal2 := spatialmath.NewPose(r3.Vector{Z: 40}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	plan, err := PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               frame.NewPoseInFrame("urCamera", goal2),
		Frame:              fs.Frame("xArmVgripper"),
		StartConfiguration: positions,
		FrameSystem:        fs,
		Options:            map[string]interface{}{"max_ik_solutions": 10, "timeout": 150.0, "smooth_iter": 5},
	})
	test.That(t, err, test.ShouldBeNil)

	// Both frames should wind up at the goal relative to one another
	solvedPose, err := fs.Transform(plan[len(plan)-1], frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()), "urCamera")
	test.That(t, err, test.ShouldBeNil)
	solvedPose2, err := fs.Transform(
		plan[len(plan)-1],
		frame.NewPoseInFrame("urCamera", spatialmath.NewZeroPose()),
		"xArmVgripper",
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal2, 0.1), test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose2.(*frame.PoseInFrame).Pose(), goal2, 0.1), test.ShouldBeTrue)
}

func TestReachOverArm(t *testing.T) {
	t.Parallel()
	// setup frame system with an xarm
	xarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	offset, err := frame.NewStaticFrame("offset", spatialmath.NewPoseFromPoint(r3.Vector{X: -500, Y: 200}))
	test.That(t, err, test.ShouldBeNil)
	goal := frame.NewPoseInFrame(
		"offset",
		spatialmath.NewPose(r3.Vector{Y: -500, Z: 100}, &spatialmath.OrientationVector{OZ: -1}),
	)
	fs := frame.NewEmptyFrameSystem("test")
	fs.AddFrame(offset, fs.World())
	fs.AddFrame(xarm, offset)

	// plan to a location, it should interpolate to get there
	opts := map[string]interface{}{"timeout": 150.0}
	plan, err := PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               goal,
		Frame:              xarm,
		StartConfiguration: frame.StartPositions(fs),
		FrameSystem:        fs,
		Options:            opts,
	})

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldEqual, 2)

	// now add a UR arm in its way
	ur5, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(ur5, fs.World())

	// the plan should no longer be able to interpolate, but it should still be able to get there
	opts = map[string]interface{}{"timeout": 150.0, "smooth_iter": 5}
	plan, err = PlanMotion(context.Background(), &PlanRequest{
		Logger:             logger.Sugar(),
		Goal:               goal,
		Frame:              xarm,
		StartConfiguration: frame.StartPositions(fs),
		FrameSystem:        fs,
		Options:            opts,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
}

func TestPlanMapMotion(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// build kinematic base model
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "base")
	test.That(t, err, test.ShouldBeNil)
	model, err := frame.New2DMobileModelFrame(
		"test",
		[]frame.Limit{{-100, 100}, {-100, 100}, {-2 * math.Pi, 2 * math.Pi}},
		sphere,
	)
	test.That(t, err, test.ShouldBeNil)
	dst := spatialmath.NewPoseFromPoint(r3.Vector{0, 100, 0})
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{0, 50, 0}), r3.Vector{25, 25, 25}, "impediment")
	test.That(t, err, test.ShouldBeNil)
	worldState, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{box})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	PlanMapMotion := func(
		ctx context.Context,
		logger golog.Logger,
		dst spatialmath.Pose,
		f frame.Frame,
		seed []frame.Input,
		worldState *frame.WorldState,
	) ([][]frame.Input, error) {
		// ephemerally create a framesystem containing just the frame for the solve
		fs := frame.NewEmptyFrameSystem("")
		if err := fs.AddFrame(f, fs.World()); err != nil {
			return nil, err
		}
		destination := frame.NewPoseInFrame(frame.World, dst)
		seedMap := map[string][]frame.Input{f.Name(): seed}
		plan, err := PlanMotion(ctx, &PlanRequest{
			Logger:             logger,
			Goal:               destination,
			Frame:              f,
			StartConfiguration: seedMap,
			FrameSystem:        fs,
			WorldState:         worldState,
		})
		if err != nil {
			return nil, err
		}
		return plan.GetFrameSteps(f.Name())
	}

	plan, err := PlanMapMotion(ctx, logger, dst, model, make([]frame.Input, 3), worldState)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
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
	t.Parallel()
	fs := makeTestFS(t)
	sf, err := newSolverFrame(fs, "xArmVgripper", frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)

	sfPlanner, err := newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	position, err := sfPlanner.PlanSingleWaypoint(
		context.Background(),
		sf.sliceToMap(make([]frame.Input, len(sf.DoF()))),
		spatialmath.NewPoseFromPoint(r3.Vector{300, 300, 100}),
		nil,
		nil,
		map[string]interface{}{"smooth_iter": 5},
	)
	test.That(t, err, test.ShouldBeNil)
	gf, _ := sf.Geometries(position[len(position)-1])
	test.That(t, gf, test.ShouldNotBeNil)

	geoms := gf.Geometries()
	for _, geom := range geoms {
		if geom.Label() == "xArmVgripper" {
			gripperCenter := geom.Pose().Point()
			test.That(t, spatialmath.R3VectorAlmostEqual(gripperCenter, r3.Vector{300, 300, 0}, 1e-2), test.ShouldBeTrue)
		}
	}
}

func TestArmConstraintSpecificationSolve(t *testing.T) {
	fs := frame.NewEmptyFrameSystem("")
	x, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(x, fs.World()), test.ShouldBeNil)
	bc, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 100}), r3.Vector{200, 200, 200}, "")
	test.That(t, err, test.ShouldBeNil)
	xArmVgripper, err := frame.NewStaticFrameWithGeometry("xArmVgripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 200}), bc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(xArmVgripper, x), test.ShouldBeNil)

	checkReachable := func(worldState *frame.WorldState, constraints *motionpb.Constraints) error {
		goal := spatialmath.NewPose(r3.Vector{X: 600, Y: 100, Z: 300}, &spatialmath.OrientationVectorDegrees{OX: 1})
		_, err := PlanMotion(context.Background(), &PlanRequest{
			Logger:             logger.Sugar(),
			Goal:               frame.NewPoseInFrame(frame.World, goal),
			Frame:              fs.Frame("xArmVgripper"),
			FrameSystem:        fs,
			StartConfiguration: frame.StartPositions(fs),
			WorldState:         worldState,
			ConstraintSpecs:    constraints,
		})
		return err
	}

	// Verify that the goal position is reachable with no obstacles
	test.That(t, checkReachable(frame.NewEmptyWorldState(), &motionpb.Constraints{}), test.ShouldBeNil)

	// Add an obstacle to the WorldState
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{350, 0, 0}), r3.Vector{10, 8000, 8000}, "theWall")
	test.That(t, err, test.ShouldBeNil)
	worldState1, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{box})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	testCases := []struct {
		name       string
		worldState *frame.WorldState
	}{
		{"obstacle specified through WorldState obstacles", worldState1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Not reachable without a collision specification
			constraints := &motionpb.Constraints{}
			err = checkReachable(tc.worldState, constraints)
			test.That(t, err, test.ShouldNotBeNil)

			// Reachable if xarm6 and gripper ignore collisions with The Wall
			constraints = &motionpb.Constraints{
				CollisionSpecification: []*motionpb.CollisionSpecification{
					{
						Allows: []*motionpb.CollisionSpecification_AllowedFrameCollisions{
							{Frame1: "xArm6", Frame2: "theWall"}, {Frame1: "xArmVgripper", Frame2: "theWall"},
						},
					},
				},
			}
			err = checkReachable(tc.worldState, constraints)
			test.That(t, err, test.ShouldBeNil)

			// Reachable if the specific bits of the xarm that collide are specified instead
			constraints = &motionpb.Constraints{
				CollisionSpecification: []*motionpb.CollisionSpecification{
					{
						Allows: []*motionpb.CollisionSpecification_AllowedFrameCollisions{
							{Frame1: "xArmVgripper", Frame2: "theWall"},
							{Frame1: "xArm6:wrist_link", Frame2: "theWall"},
							{Frame1: "xArm6:lower_forearm", Frame2: "theWall"},
						},
					},
				},
			}
			err = checkReachable(tc.worldState, constraints)
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func TestMovementWithGripper(t *testing.T) {
	// TODO(rb): move these tests to a separate repo eventually, as they take up too much time for general CI pipeline
	t.Skip()

	// setup solverFrame and planning query
	fs := makeTestFS(t)
	fs.RemoveFrame(fs.Frame("urOffset"))
	sf, err := newSolverFrame(fs, "xArmVgripper", frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	goal := spatialmath.NewPose(r3.Vector{500, 0, -300}, &spatialmath.OrientationVector{OZ: -1})
	zeroPosition := sf.sliceToMap(make([]frame.Input, len(sf.DoF())))

	// linearly plan with the gripper
	motionConfig := make(map[string]interface{})
	motionConfig["motion_profile"] = LinearMotionProfile
	sfPlanner, err := newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err := sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, nil, nil, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// plan around the obstacle with the gripper
	obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{300, 0, -400}), r3.Vector{50, 500, 500}, "")
	test.That(t, err, test.ShouldBeNil)
	worldState, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{obstacle})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// plan with end of arm with gripper attached - this will fail
	sf, err = newSolverFrame(fs, "xArm6", frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	goal = spatialmath.NewPose(r3.Vector{500, 0, -100}, &spatialmath.OrientationVector{OZ: -1})
	zeroPosition = sf.sliceToMap(make([]frame.Input, len(sf.DoF())))
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	_, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil, motionConfig)
	test.That(t, err, test.ShouldNotBeNil)

	// remove linear constraint and try again
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// remove gripper and try with linear constraint
	fs.RemoveFrame(fs.Frame("xArmVgripper"))
	sf, err = newSolverFrame(fs, "xArm6", frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	zeroPosition = sf.sliceToMap(make([]frame.Input, len(sf.DoF())))
	sfPlanner, err = newPlanManager(sf, fs, logger.Sugar(), 1)
	test.That(t, err, test.ShouldBeNil)
	solution, err = sfPlanner.PlanSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)
}

func TestReplan(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "base")
	test.That(t, err, test.ShouldBeNil)

	kinematicFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"itsabase",
		logger,
		200, 60, 0, 1000,
		2,
		[]spatialmath.Geometry{sphere},
		false,
	)
	test.That(t, err, test.ShouldBeNil)

	goal := spatialmath.NewPoseFromPoint(r3.Vector{1000, 8000, 0})

	baseFS := frame.NewEmptyFrameSystem("baseFS")
	err = baseFS.AddFrame(kinematicFrame, baseFS.World())
	test.That(t, err, test.ShouldBeNil)

	planRequest := &PlanRequest{
		Logger:             logger,
		Goal:               frame.NewPoseInFrame(frame.World, goal),
		Frame:              kinematicFrame,
		FrameSystem:        baseFS,
		StartConfiguration: frame.StartPositions(baseFS),
		WorldState:         nil,
		Options:            nil,
	}

	firstplan, err := PlanMotion(ctx, planRequest)
	test.That(t, err, test.ShouldBeNil)

	// Let's pretend we've moved towards the goal, so the goal is now closer
	goal = spatialmath.NewPoseFromPoint(r3.Vector{1000, 5000, 0})
	planRequest.Goal = frame.NewPoseInFrame(frame.World, goal)

	// This should easily pass
	newPlan1, err := Replan(ctx, planRequest, firstplan, 1.0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(newPlan1), test.ShouldBeGreaterThan, 2)

	// But if we drop the replan factor to a very low number, it should now fail
	newPlan2, err := Replan(ctx, planRequest, firstplan, 0.1)
	test.That(t, newPlan2, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, errHighReplanCost) // Replan factor too low!
}

func TestValidatePlanRequest(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name        string
		request     PlanRequest
		expectedErr error
	}

	logger := golog.NewTestLogger(t)
	fs := frame.NewEmptyFrameSystem("test")
	frame1 := frame.NewZeroStaticFrame("frame1")
	frame2, err := frame.NewTranslationalFrame("frame2", r3.Vector{1, 0, 0}, frame.Limit{1, 1})
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame1, fs.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame2, fs.World())
	test.That(t, err, test.ShouldBeNil)

	validGoal := frame.NewPoseInFrame("frame1", spatialmath.NewZeroPose())
	badGoal := frame.NewPoseInFrame("non-existent", spatialmath.NewZeroPose())

	testCases := []testCase{
		{
			name:        "empty request - fail",
			request:     PlanRequest{},
			expectedErr: errors.New("PlanRequest cannot have nil logger"),
		},
		{
			name: "nil frame - fail",
			request: PlanRequest{
				Logger: logger,
			},
			expectedErr: errors.New("PlanRequest cannot have nil frame"),
		},
		{
			name: "nil framesystem - fail",
			request: PlanRequest{
				Logger: logger,
				Frame:  frame1,
			},
			expectedErr: errors.New("PlanRequest cannot have nil framesystem"),
		},
		{
			name: "framesystem does not contain frame - fail",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame1,
				FrameSystem: frame.NewEmptyFrameSystem("test"),
			},
			expectedErr: errors.Errorf("frame with name %q not in frame system", frame1.Name()),
		},
		{
			name: "nil goal - fail",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame1,
				FrameSystem: fs,
			},
			expectedErr: errors.New("PlanRequest cannot have nil goal"),
		},
		{
			name: "goal's parent not in frame system - fail",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame1,
				FrameSystem: fs,
				Goal:        badGoal,
			},
			expectedErr: errors.New("part with name  references non-existent parent non-existent"),
		},
		{
			name: "incorrect length StartConfiguration - fail",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame1,
				FrameSystem: fs,
				Goal:        validGoal,
				StartConfiguration: map[string][]frame.Input{
					"frame1": frame.FloatsToInputs([]float64{0}),
				},
			},
			expectedErr: errors.New("number of inputs does not match frame DoF, expected 0 but got 1"),
		},
		{
			name: "incorrect length StartConfiguration - fail",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame2,
				FrameSystem: fs,
				Goal:        validGoal,
			},
			expectedErr: errors.New("frame2 does not have a start configuration"),
		},
		{
			name: "incorrect length StartConfiguration - fail",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame2,
				FrameSystem: fs,
				Goal:        validGoal,
				StartConfiguration: map[string][]frame.Input{
					"frame2": frame.FloatsToInputs([]float64{0, 0, 0, 0, 0}),
				},
			},
			expectedErr: errors.New("number of inputs does not match frame DoF, expected 1 but got 5"),
		},
		{
			name: "well formed PlanRequest",
			request: PlanRequest{
				Logger:      logger,
				Frame:       frame1,
				FrameSystem: fs,
				Goal:        validGoal,
				StartConfiguration: map[string][]frame.Input{
					"frame1": {},
				},
			},
			expectedErr: nil,
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		err := tc.request.validatePlanRequest()
		if tc.expectedErr != nil {
			test.That(t, err.Error(), test.ShouldEqual, tc.expectedErr.Error())
		} else {
			test.That(t, err, test.ShouldBeNil)
		}
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testFn(t, c)
		})
	}

	// ensure nil PlanRequests are caught
	_, err = PlanMotion(context.Background(), nil)
	test.That(t, err.Error(), test.ShouldEqual, "PlanRequest cannot be nil")
}
