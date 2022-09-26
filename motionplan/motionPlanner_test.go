package motionplan

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/zap"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
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
	Goal       *commonpb.Pose
	RobotFrame frame.Frame
	Options    *PlannerOptions
}

type (
	seededPlannerConstructor func(frame frame.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error)
	planConfigConstructor    func() (*planConfig, error)
)

func BenchmarkUnconstrainedMotion(b *testing.B) {
	config, err := simpleUR5eMotion()
	test.That(b, err, test.ShouldBeNil)
	mp, err := NewRRTConnectMotionPlannerWithSeed(config.RobotFrame, nCPU/4, rand.New(rand.NewSource(int64(1))), logger.Sugar())
	test.That(b, err, test.ShouldBeNil)
	plan, err := mp.Plan(context.Background(), config.Goal, config.Start, config.Options)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, len(plan), test.ShouldBeGreaterThanOrEqualTo, 2)
}

func TestUnconstrainedMotion(t *testing.T) {
	t.Parallel()
	planners := []seededPlannerConstructor{
		NewRRTStarConnectMotionPlannerWithSeed,
		NewRRTConnectMotionPlannerWithSeed,
		NewCBiRRTMotionPlannerWithSeed,
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
			for _, planner := range planners {
				testPlanner(t, planner, testCase.config, 1)
			}
		})
	}
}

func TestConstrainedMotion(t *testing.T) {
	t.Parallel()
	planners := []seededPlannerConstructor{
		NewCBiRRTMotionPlannerWithSeed,
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
			for _, planner := range planners {
				testPlanner(t, planner, testCase.config, 1)
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
	pos := &commonpb.Pose{X: -206, Y: 100, Z: 120, OZ: -1}

	opt := NewBasicPlannerOptions()
	orientMetric := NewPoseFlexOVMetric(spatial.NewPoseFromProtobuf(pos), 0.09)

	oFunc := orientDistToRegion(spatial.NewPoseFromProtobuf(pos).Orientation(), 0.1)
	oFuncMet := func(from, to spatial.Pose) float64 {
		return oFunc(from.Orientation())
	}
	orientConstraint := func(o spatial.Orientation) bool {
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
	bc, _ := spatial.NewBoxCreator(r3.Vector{200, 200, 200}, spatial.NewPoseFromPoint(r3.Vector{Z: 75}))
	gripper, err := frame.NewStaticFrameWithGeometry("gripper", spatial.NewPoseFromPoint(r3.Vector{Z: 150}), bc)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gripper, ur5e)
	test.That(t, err, test.ShouldBeNil)
	fss := NewSolvableFrameSystem(fs, logger.Sugar())
	zeroPos := frame.StartPositions(fss)

	newPose := frame.NewPoseInFrame("gripper", spatial.NewPoseFromPoint(r3.Vector{100, 100, 0}))
	solutionMap, err := fss.SolvePose(context.Background(), zeroPos, newPose, gripper.Name())
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
	physicalGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 10, Y: 10, Z: 10}, spatial.NewZeroPose())
	if err != nil {
		return nil, err
	}
	model, err := frame.NewMobile2DFrame("mobile-base", limits, physicalGeometry)
	if err != nil {
		return nil, err
	}

	// obstacles
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 50, 0}), r3.Vector{80, 80, 1})
	if err != nil {
		return nil, err
	}

	// setup planner options
	opt := NewBasicPlannerOptions()
	toMap := func(geometries []spatial.Geometry) map[string]spatial.Geometry {
		geometryMap := make(map[string]spatial.Geometry, 0)
		for i, geometry := range geometries {
			geometryMap[strconv.Itoa(i)] = geometry
		}
		return geometryMap
	}
	startInput := frame.FloatsToInputs([]float64{-90., 90.})
	opt.AddConstraint("collision", NewCollisionConstraint(model, startInput, toMap([]spatial.Geometry{box}), nil))

	return &planConfig{
		Start:      startInput,
		Goal:       spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: 90, Y: 90, Z: 0})),
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

	// setup planner options
	opt := NewBasicPlannerOptions()
	opt.AddConstraint("collision", NewCollisionConstraint(xarm, home7, nil, nil))

	return &planConfig{
		Start:      home7,
		Goal:       &commonpb.Pose{X: 206, Y: 100, Z: 120, OZ: -1},
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

	// setup planner options
	opt := NewBasicPlannerOptions()
	opt.AddConstraint("collision", NewCollisionConstraint(ur5e, home6, nil, nil))

	return &planConfig{
		Start:      home6,
		Goal:       &commonpb.Pose{X: -750, Y: -250, Z: 200, OX: -1},
		RobotFrame: ur5e,
		Options:    opt,
	}, nil
}

// testPlanner is a helper function that takes a planner and a planning query specified through a config object and tests that it
// returns a valid set of waypoints.
func testPlanner(t *testing.T, planner seededPlannerConstructor, config planConfigConstructor, seed int) {
	t.Helper()

	// plan
	cfg, err := config()
	test.That(t, err, test.ShouldBeNil)
	mp, err := planner(cfg.RobotFrame, nCPU/4, rand.New(rand.NewSource(int64(seed))), logger.Sugar())
	test.That(t, err, test.ShouldBeNil)
	path, err := mp.Plan(context.Background(), cfg.Goal, cfg.Start, cfg.Options)
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
