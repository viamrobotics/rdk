package armplanning

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home7 = []frame.Input{0, 0, 0, 0, 0, 0, 0}
	home6 = []frame.Input{0, 0, 0, 0, 0, 0}
)

type planConfig struct {
	Start            *PlanState
	Goal             *PlanState
	FS               *frame.FrameSystem
	Options          *PlannerOptions
	ConstraintHander *motionplan.ConstraintChecker
	MotionChains     *motionChains
	Constraints      *motionplan.Constraints
}

type planConfigConstructor func(logger logging.Logger) (*planConfig, error)

func TestUnconstrainedMotion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCases := []struct {
		name   string
		config planConfigConstructor
	}{
		{"6D plan test", simpleUR5eMotion},
		{"7D plan test", simpleXArmMotion},
	}
	for _, testCase := range testCases {
		tcCopy := testCase
		t.Run(tcCopy.name, func(t *testing.T) {
			t.Parallel()
			testPlanner(t, ctx, tcCopy.config)
		})
	}
}

func TestConstrainedMotion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
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
			testPlanner(t, ctx, tcCopy.config)
		})
	}
}

// TestConstrainedArmMotion tests a simple linear motion on a longer path, with a no-spill constraint.
func constrainedXArmMotion(logger logging.Logger) (*planConfig, error) {
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm7.json"), "")
	if err != nil {
		return nil, err
	}

	// Test ability to arrive at another position
	pos := spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: -206, Y: 100, Z: 120, OZ: -1})

	opt := NewBasicPlannerOptions()
	// Create a temporary frame system for the transformation
	fs := frame.NewEmptyFrameSystem("")
	err = fs.AddFrame(model, fs.World())
	if err != nil {
		return nil, err
	}

	cons := motionplan.NewEmptyConstraints()
	cons.OrientationConstraint = append(cons.OrientationConstraint, motionplan.OrientationConstraint{1})

	start := &PlanState{structuredConfiguration: map[string][]frame.Input{model.Name(): home7}}
	goalPoses := frame.FrameSystemPoses{model.Name(): frame.NewPoseInFrame(frame.World, pos)}
	goal := &PlanState{poses: goalPoses}
	motionChains, err := motionChainsFromPlanState(fs, goalPoses)
	if err != nil {
		return nil, err
	}

	return &planConfig{
		Start:            start,
		Goal:             goal,
		FS:               fs,
		Options:          opt,
		ConstraintHander: motionplan.NewEmptyConstraintChecker(),
		MotionChains:     motionChains,
		Constraints:      cons,
	}, nil
}

func TestPlanningWithGripper(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := frame.NewEmptyFrameSystem("")
	ur5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "ur")
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(ur5e, fs.World())
	test.That(t, err, test.ShouldBeNil)
	bc, _ := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 75}), r3.Vector{200, 200, 200}, "")
	gripper, err := frame.NewStaticFrameWithGeometry("gripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 150}), bc)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gripper, ur5e)
	test.That(t, err, test.ShouldBeNil)
	zeroPos := frame.NewZeroInputs(fs)

	newPose := frame.NewPoseInFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{100, 100, 0}))
	solutionMap, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{"gripper": newPose}}},
		StartState:     &PlanState{structuredConfiguration: zeroPos},
		PlannerOptions: NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(solutionMap.Trajectory()), test.ShouldBeGreaterThanOrEqualTo, 2)
}

// simpleArmMotion tests moving an xArm7.
func simpleXArmMotion(logger logging.Logger) (*planConfig, error) {
	xarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm7.json"), "")
	if err != nil {
		return nil, err
	}

	// add it to the frame system
	fs := frame.NewEmptyFrameSystem("test")
	if err := fs.AddFrame(xarm, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	goal := &PlanState{poses: frame.FrameSystemPoses{
		xarm.Name(): frame.NewPoseInFrame(frame.World, spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: 206, Y: 100, Z: 120, OZ: -1})),
	}}

	// setup planner options
	opt := NewBasicPlannerOptions()

	// create robot collision entities
	movingGeometriesInFrame, err := xarm.Geometries(home7)
	if err != nil {
		return nil, err
	}
	movingRobotGeometries := movingGeometriesInFrame.Geometries()

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatialmath.Geometry, 0)
	frameSystemGeometries, err := frame.FrameSystemGeometries(fs, frame.NewZeroInputs(fs))
	if err != nil {
		return nil, err
	}
	for name, geometries := range frameSystemGeometries {
		if name != xarm.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	fsCollisionConstraints, err := motionplan.CreateAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		nil,
		nil,
		defaultCollisionBufferMM,
	)
	if err != nil {
		return nil, err
	}

	constraintHandler := motionplan.NewEmptyConstraintChecker()
	for name, constraint := range fsCollisionConstraints {
		constraintHandler.AddStateFSConstraint(name, constraint)
	}
	start := map[string][]frame.Input{xarm.Name(): home7}
	motionChains, err := motionChainsFromPlanState(fs, goal.poses)
	if err != nil {
		return nil, err
	}

	return &planConfig{
		Start:            &PlanState{structuredConfiguration: start},
		Goal:             goal,
		FS:               fs,
		Options:          opt,
		ConstraintHander: constraintHandler,
		MotionChains:     motionChains,
	}, nil
}

// simpleUR5eMotion tests a simple motion for a UR5e.
func simpleUR5eMotion(logger logging.Logger) (*planConfig, error) {
	ur5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
	if err != nil {
		return nil, err
	}

	fs := frame.NewEmptyFrameSystem("test")
	if err = fs.AddFrame(ur5e, fs.Frame(frame.World)); err != nil {
		return nil, err
	}

	goal := &PlanState{poses: frame.FrameSystemPoses{
		ur5e.Name(): frame.NewPoseInFrame(frame.World, spatialmath.NewPoseFromProtobuf(&commonpb.Pose{X: -750, Y: -250, Z: 200, OX: -1})),
	}}

	// setup planner options
	opt := NewBasicPlannerOptions()

	// create robot collision entities
	movingGeometriesInFrame, err := ur5e.Geometries(home6)
	if err != nil {
		return nil, err
	}
	movingRobotGeometries := movingGeometriesInFrame.Geometries()

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatialmath.Geometry, 0)
	frameSystemGeometries, err := frame.FrameSystemGeometries(fs, frame.NewZeroInputs(fs))
	if err != nil {
		return nil, err
	}
	for name, geometries := range frameSystemGeometries {
		if name != ur5e.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	fsCollisionConstraints, err := motionplan.CreateAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		nil,
		nil,
		defaultCollisionBufferMM,
	)
	if err != nil {
		return nil, err
	}
	constraintHandler := motionplan.NewEmptyConstraintChecker()
	for name, constraint := range fsCollisionConstraints {
		constraintHandler.AddStateFSConstraint(name, constraint)
	}
	start := map[string][]frame.Input{ur5e.Name(): home6}
	motionChains, err := motionChainsFromPlanState(fs, goal.poses)
	if err != nil {
		return nil, err
	}

	return &planConfig{
		Start:            &PlanState{structuredConfiguration: start},
		Goal:             goal,
		FS:               fs,
		Options:          opt,
		ConstraintHander: constraintHandler,
		MotionChains:     motionChains,
	}, nil
}

// testPlanner is a helper function that takes a planner and a planning query specified through a config object and tests that it
// returns a valid set of waypoints.
func testPlanner(t *testing.T, ctx context.Context, config planConfigConstructor) {
	t.Helper()
	logger := logging.NewTestLogger(t)

	// plan
	cfg, err := config(logger)
	test.That(t, err, test.ShouldBeNil)

	// Create PlanRequest to use the new API
	request := &PlanRequest{
		FrameSystem:    cfg.FS,
		Goals:          []*PlanState{cfg.Goal},
		StartState:     cfg.Start,
		PlannerOptions: cfg.Options,
		Constraints:    cfg.Constraints,
	}

	pc, err := newPlanContext(ctx, logger, request, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, cfg.Start.LinearConfiguration(), cfg.Goal.poses)
	test.That(t, err, test.ShouldBeNil)

	mp, err := newCBiRRTMotionPlanner(ctx, pc, psc)
	test.That(t, err, test.ShouldBeNil)

	nodes, err := mp.planForTest(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// test that path doesn't violate constraints
	test.That(t, len(nodes), test.ShouldBeGreaterThanOrEqualTo, 2)
	for j := 0; j < len(nodes)-1; j++ {
		_, err := cfg.ConstraintHander.CheckStateConstraintsAcrossSegmentFS(
			ctx,
			&motionplan.SegmentFS{
				StartConfiguration: nodes[j],
				EndConfiguration:   nodes[j+1],
				FS:                 cfg.FS,
			}, cfg.Options.Resolution)
		test.That(t, err, test.ShouldBeNil)
	}
}

func makeTestFS(t *testing.T) *frame.FrameSystem {
	t.Helper()
	fs := frame.NewEmptyFrameSystem("test")

	urOffset, err := frame.NewStaticFrame("urOffset", spatialmath.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urOffset, fs.World())
	gantryOffset, err := frame.NewStaticFrame("gantryOffset", spatialmath.NewPoseFromPoint(r3.Vector{-50, -50, -200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	gantryX, err := frame.NewTranslationalFrame("gantryX", r3.Vector{1, 0, 0}, frame.Limit{-1000000, 100000})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryX, gantryOffset)
	gantryY, err := frame.NewTranslationalFrame("gantryY", r3.Vector{0, 1, 0}, frame.Limit{-100000, 100000})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryY, gantryX)

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantryY)

	modelUR5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
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

func TestSerializedPlanRequest(t *testing.T) {
	fs := frame.NewEmptyFrameSystem("")
	x, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(x, fs.World()), test.ShouldBeNil)
	bc, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 100}), r3.Vector{200, 200, 200}, "")
	test.That(t, err, test.ShouldBeNil)
	xArmVgripper, err := frame.NewStaticFrameWithGeometry("xArmVgripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 200}), bc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(xArmVgripper, x), test.ShouldBeNil)

	constraints := &motionplan.Constraints{
		CollisionSpecification: []motionplan.CollisionSpecification{
			{
				Allows: []motionplan.CollisionSpecificationAllowedFrameCollisions{
					{Frame1: "xArmVgripper", Frame2: "theWall"},
					{Frame1: "xArm6:wrist_link", Frame2: "theWall"},
					{Frame1: "xArm6:lower_forearm", Frame2: "theWall"},
				},
			},
		},
	}

	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{350, 0, 0}), r3.Vector{10, 8000, 8000}, "theWall")
	test.That(t, err, test.ShouldBeNil)
	worldState1, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{box})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	goal := spatialmath.NewPose(r3.Vector{X: 600, Y: 100, Z: 300}, &spatialmath.OrientationVectorDegrees{OX: 1})

	pr := &PlanRequest{
		FrameSystem: fs,
		Goals:       []*PlanState{{poses: frame.FrameSystemPoses{"xArmVgripper": frame.NewPoseInFrame(frame.World, goal)}}},
		StartState:  &PlanState{structuredConfiguration: frame.NewZeroInputs(fs)},
		WorldState:  worldState1,
		Constraints: constraints,
	}

	jsonData, err := os.ReadFile("data/plan_request_sample.json")
	test.That(t, err, test.ShouldBeNil)
	parsedPr := &PlanRequest{}
	err = json.Unmarshal(jsonData, parsedPr)
	test.That(t, err, test.ShouldBeNil)

	goalPose1 := pr.Goals[0].Poses()["xArmVgripper"].Pose()
	goalPoseInFrame2, ok := parsedPr.Goals[0].Poses()["xArmVgripper"]
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(goalPose1, goalPoseInFrame2.Pose()), test.ShouldBeTrue)

	collisionSpecification1 := pr.Constraints.CollisionSpecification[0]
	test.That(t, parsedPr.Constraints, test.ShouldNotBeNil)
	collisionSpecification2 := parsedPr.Constraints.CollisionSpecification[0]
	allows1 := collisionSpecification1.Allows
	allows2 := collisionSpecification2.Allows
	test.That(t, allows1[0].Frame1, test.ShouldEqual, allows2[0].Frame1)
	test.That(t, allows1[0].Frame2, test.ShouldEqual, allows2[0].Frame2)

	test.That(t, allows1[1].Frame1, test.ShouldEqual, allows2[1].Frame1)
	test.That(t, allows1[1].Frame2, test.ShouldEqual, allows2[1].Frame2)

	test.That(t, allows1[2].Frame1, test.ShouldEqual, allows2[2].Frame1)
	test.That(t, allows1[2].Frame2, test.ShouldEqual, allows2[2].Frame2)

	startStateConf1 := pr.StartState.Configuration()["xArm6"]
	test.That(t, parsedPr.StartState, test.ShouldNotBeNil)
	startStateConfColl2 := parsedPr.StartState.Configuration()
	startStateConf2, ok := startStateConfColl2["xArm6"]
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, startStateConf1, test.ShouldResemble, startStateConf2)

	geometryIF1 := pr.WorldState.Obstacles()[0]
	test.That(t, parsedPr.WorldState, test.ShouldNotBeNil)
	test.That(t, parsedPr.WorldState.Obstacles(), test.ShouldNotBeEmpty)
	geometryIF2 := parsedPr.WorldState.Obstacles()[0]
	test.That(t, geometryIF1.Parent(), test.ShouldEqual, geometryIF2.Parent())
	geometries1 := geometryIF1.Geometries()
	geometries2 := geometryIF2.Geometries()
	test.That(t, len(geometries1), test.ShouldEqual, len(geometries2))
	geometry1 := geometries1[0]
	geometry2 := geometries2[0]
	test.That(t, spatialmath.PoseAlmostEqual(geometry1.Pose(), geometry2.Pose()), test.ShouldBeTrue)

	fs1 := pr.FrameSystem
	fs2 := parsedPr.FrameSystem
	frames1 := fs1.FrameNames()
	frames2 := fs2.FrameNames()
	sort.Strings(frames1)
	sort.Strings(frames2)
	test.That(t, frames1, test.ShouldResemble, frames2)
}

func TestArmOOBSolve(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := makeTestFS(t)
	positions := frame.NewZeroInputs(fs)

	// Set a goal unreachable by the UR due to sheer distance
	goal1 := spatialmath.NewPose(r3.Vector{X: 257, Y: 21000, Z: -300}, &spatialmath.OrientationVectorDegrees{OZ: -1})
	_, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{"urCamera": frame.NewPoseInFrame(frame.World, goal1)}}},
		StartState:     &PlanState{structuredConfiguration: positions},
		PlannerOptions: NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, errIKSolve.Error())
}

func TestArmObstacleSolve(t *testing.T) {
	logger := logging.NewTestLogger(t)

	fs := makeTestFS(t)
	positions := frame.NewZeroInputs(fs)

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
	_, _, err = PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{"urCamera": frame.NewPoseInFrame(frame.World, goal1)}}},
		StartState:     &PlanState{structuredConfiguration: positions},
		WorldState:     worldState,
		PlannerOptions: NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "constraint")
}

func TestArmAndGantrySolve(t *testing.T) {
	if Is32Bit() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)
	t.Parallel()
	fs := makeTestFS(t)
	positions := frame.NewZeroLinearInputs(fs)
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
	planOpts, err := NewPlannerOptionsFromExtra(map[string]interface{}{"smooth_iter": 5})
	test.That(t, err, test.ShouldBeNil)
	plan, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{"xArmVgripper": frame.NewPoseInFrame(frame.World, goal1)}}},
		StartState:     &PlanState{structuredConfiguration: positions.ToFrameSystemInputs()},
		PlannerOptions: planOpts,
	})
	test.That(t, err, test.ShouldBeNil)
	solvedPose, err := fs.Transform(
		plan.Trajectory()[len(plan.Trajectory())-1].ToLinearInputs(),
		frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()),
		frame.World,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal1, 0.01), test.ShouldBeTrue)
}

func TestMultiArmSolve(t *testing.T) {
	if Is32Bit() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)
	fs := makeTestFS(t)
	positions := frame.NewZeroInputs(fs)

	// Solve such that the ur5 and xArm are pointing at each other, 40mm from gripper to camera
	goals := frame.FrameSystemPoses{
		"xArmVgripper": frame.NewPoseInFrame("urCamera", spatialmath.NewPose(r3.Vector{Z: 60}, &spatialmath.OrientationVectorDegrees{OZ: -1})),
	}
	goals, err := translateGoalsToWorldPosition(fs, positions.ToLinearInputs(), goals)
	test.That(t, err, test.ShouldBeNil)

	planOpts, err := NewPlannerOptionsFromExtra(
		map[string]interface{}{"max_ik_solutions": 10, "timeout": 150.0, "smooth_iter": 5},
	)
	test.That(t, err, test.ShouldBeNil)
	plan, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: goals}},
		StartState:     &PlanState{structuredConfiguration: positions},
		PlannerOptions: planOpts,
	})
	test.That(t, err, test.ShouldBeNil)

	// Both frames should wind up at the goal relative to one another
	solvedPose, err := fs.Transform(
		plan.Trajectory()[len(plan.Trajectory())-1].ToLinearInputs(),
		frame.NewPoseInFrame("xArmVgripper", spatialmath.NewZeroPose()),
		"world",
	)
	test.That(t, err, test.ShouldBeNil)

	test.That(t,
		spatialmath.PoseAlmostCoincidentEps(
			solvedPose.(*frame.PoseInFrame).Pose(),
			goals["xArmVgripper"].Pose(), 0.1),
		test.ShouldBeTrue)
}

func TestReachOverArm(t *testing.T) {
	// setup frame system with an xarm
	xarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
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
	planOpts, err := NewPlannerOptionsFromExtra(opts)
	test.That(t, err, test.ShouldBeNil)

	logger := logging.NewTestLogger(t)
	plan, _, err := PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{xarm.Name(): goal}}},
		StartState:     &PlanState{structuredConfiguration: frame.NewZeroInputs(fs)},
		PlannerOptions: planOpts,
	})

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan.Trajectory()), test.ShouldEqual, 2)

	// now add a UR arm in its way
	ur5, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(ur5, fs.World())

	// the plan should no longer be able to interpolate, but it should still be able to get there
	opts = map[string]interface{}{"timeout": 150.0, "smooth_iter": 5}
	planOpts, err = NewPlannerOptionsFromExtra(opts)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = PlanMotion(context.Background(), logger, &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{xarm.Name(): goal}}},
		StartState:     &PlanState{structuredConfiguration: frame.NewZeroInputs(fs)},
		PlannerOptions: planOpts,
	})
	test.That(t, err, test.ShouldBeNil)
}

func TestPlanMapMotion(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

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
		logger logging.Logger,
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
		plan, _, err := PlanMotion(ctx, logger, &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{poses: frame.FrameSystemPoses{f.Name(): destination}}},
			StartState:     &PlanState{structuredConfiguration: seedMap},
			WorldState:     worldState,
			PlannerOptions: NewBasicPlannerOptions(),
		})
		if err != nil {
			return nil, err
		}
		return plan.Trajectory().GetFrameInputs(f.Name())
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

func TestArmConstraintSpecificationSolve(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := frame.NewEmptyFrameSystem("")
	x, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(x, fs.World()), test.ShouldBeNil)
	bc, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 100}), r3.Vector{200, 200, 200}, "")
	test.That(t, err, test.ShouldBeNil)
	xArmVgripper, err := frame.NewStaticFrameWithGeometry("xArmVgripper", spatialmath.NewPoseFromPoint(r3.Vector{Z: 200}), bc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(xArmVgripper, x), test.ShouldBeNil)

	checkReachable := func(worldState *frame.WorldState, constraints *motionplan.Constraints) error {
		goal := spatialmath.NewPose(r3.Vector{X: 600, Y: 100, Z: 300}, &spatialmath.OrientationVectorDegrees{OX: 1})
		_, _, err = PlanMotion(context.Background(), logger, &PlanRequest{
			FrameSystem:    fs,
			Goals:          []*PlanState{{poses: frame.FrameSystemPoses{"xArmVgripper": frame.NewPoseInFrame(frame.World, goal)}}},
			StartState:     &PlanState{structuredConfiguration: frame.NewZeroInputs(fs)},
			WorldState:     worldState,
			Constraints:    constraints,
			PlannerOptions: NewBasicPlannerOptions(),
		})
		return err
	}

	// Verify that the goal position is reachable with no obstacles
	test.That(t, checkReachable(frame.NewEmptyWorldState(), &motionplan.Constraints{}), test.ShouldBeNil)

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
			constraints := &motionplan.Constraints{}
			err = checkReachable(tc.worldState, constraints)
			test.That(t, err, test.ShouldNotBeNil)

			// Reachable if xarm6 and gripper ignore collisions with The Wall
			constraints = &motionplan.Constraints{
				CollisionSpecification: []motionplan.CollisionSpecification{
					{
						Allows: []motionplan.CollisionSpecificationAllowedFrameCollisions{
							{Frame1: "xArm6", Frame2: "theWall"}, {Frame1: "xArmVgripper", Frame2: "theWall"},
						},
					},
				},
			}
			err = checkReachable(tc.worldState, constraints)
			test.That(t, err, test.ShouldBeNil)

			// Reachable if the specific bits of the xarm that collide are specified instead
			constraints = &motionplan.Constraints{
				CollisionSpecification: []motionplan.CollisionSpecification{
					{
						Allows: []motionplan.CollisionSpecificationAllowedFrameCollisions{
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

func TestValidatePlanRequest(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name        string
		request     *PlanRequest
		expectedErr error
	}

	fs := frame.NewEmptyFrameSystem("test")
	frame1 := frame.NewZeroStaticFrame("frame1")
	frame2, err := frame.NewTranslationalFrame("frame2", r3.Vector{1, 0, 0}, frame.Limit{-1, 1})
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame1, fs.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame2, fs.World())
	test.That(t, err, test.ShouldBeNil)

	validGoal := []*PlanState{{
		poses: frame.FrameSystemPoses{"frame1": frame.NewPoseInFrame("frame1", spatialmath.NewZeroPose())},
	}}
	badGoal := []*PlanState{{
		poses: frame.FrameSystemPoses{"frame1": frame.NewPoseInFrame("non-existent", spatialmath.NewZeroPose())},
	}}

	testCases := []testCase{
		{
			name: "absent start state - fail",
			request: &PlanRequest{
				FrameSystem: fs,
				Goals:       validGoal,
			},
			expectedErr: errors.New("PlanRequest cannot have nil StartState"),
		},
		{
			name: "nil goal - fail",
			request: &PlanRequest{
				FrameSystem: fs,
				StartState: &PlanState{structuredConfiguration: map[string][]frame.Input{
					"frame1": {}, "frame2": {0},
				}},
			},
			expectedErr: errors.New("PlanRequest must have at least one goal"),
		},
		{
			name: "goal's parent not in frame system - fail",
			request: &PlanRequest{
				FrameSystem: fs,
				Goals:       badGoal,
				StartState: &PlanState{structuredConfiguration: map[string][]frame.Input{
					"frame1": {}, "frame2": {0},
				}},
				PlannerOptions: NewBasicPlannerOptions(),
			},
			expectedErr: errors.New("part with name frame1 references non-existent parent non-existent"),
		},
		{
			name: "absent StartState Configuration - fail",
			request: &PlanRequest{
				FrameSystem: fs,
				Goals:       validGoal,
				StartState:  &PlanState{},
			},
			expectedErr: errors.New("PlanRequest cannot have nil StartState configuration"),
		},
		{
			name: "incorrect length StartConfiguration - fail",
			request: &PlanRequest{
				FrameSystem: fs,
				Goals:       validGoal,
				StartState: &PlanState{structuredConfiguration: map[string][]frame.Input{
					"frame1": {}, "frame2": {0, 0, 0, 0, 0},
				}},
				PlannerOptions: NewBasicPlannerOptions(),
			},
			expectedErr: frame.NewIncorrectDoFError(5, 1),
		},
		{
			name: "well formed PlanRequest",
			request: &PlanRequest{
				FrameSystem: fs,
				Goals:       validGoal,
				StartState: &PlanState{structuredConfiguration: map[string][]frame.Input{
					"frame1": {},
					"frame2": {0},
				}},
				PlannerOptions: NewBasicPlannerOptions(),
			},
			expectedErr: nil,
		},
		{
			name:        "nil framesystem errors correctly",
			request:     &PlanRequest{},
			expectedErr: errors.New("PlanRequest cannot have nil framesystem"),
		},
		{
			name:        "nil PlanRequest errors correctly",
			request:     nil,
			expectedErr: errors.New("PlanRequest cannot be nil"),
		},
		{
			name: "nil PlannerOptions does not fail",
			request: &PlanRequest{
				FrameSystem: fs,
				Goals:       validGoal,
				StartState: &PlanState{structuredConfiguration: map[string][]frame.Input{
					"frame1": {},
					"frame2": {0},
				}},
				PlannerOptions: nil,
			},
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		err := tc.request.validatePlanRequest()
		if tc.expectedErr != nil {
			test.That(t, err, test.ShouldNotBeNil)
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
}

func TestArmGantryCheckPlan(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := frame.NewEmptyFrameSystem("test")

	gantryOffset, err := frame.NewStaticFrame("gantryOffset", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gantryOffset, fs.World())
	test.That(t, err, test.ShouldBeNil)

	lim := frame.Limit{Min: math.Inf(-1), Max: math.Inf(1)}
	gantryX, err := frame.NewTranslationalFrame("gantryX", r3.Vector{1, 0, 0}, lim)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gantryX, gantryOffset)
	test.That(t, err, test.ShouldBeNil)

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(modelXarm, gantryX)
	test.That(t, err, test.ShouldBeNil)

	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 407, Y: 0, Z: 112})

	planReq := PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{{poses: frame.FrameSystemPoses{"xArm6": frame.NewPoseInFrame(frame.World, goal)}}},
		StartState:     &PlanState{structuredConfiguration: frame.NewZeroInputs(fs)},
		PlannerOptions: NewBasicPlannerOptions(),
	}

	_, _, err = PlanMotion(context.Background(), logger, &planReq)
	test.That(t, err, test.ShouldBeNil)
}
