package motionplan

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/zap"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	home6 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
)

var logger, err = zap.Config{
	Level:             zap.NewAtomicLevelAt(zap.FatalLevel),
	Encoding:          "console",
	DisableStacktrace: true,
}.Build()

type planConfig struct {
	NumTests          int
	Start             []frame.Input
	Goal              *commonpb.Pose
	RobotFrame        frame.Frame
	Obstacles         []spatial.Geometry
	InteractionSpaces []spatial.Geometry
}

type seededPlannerConstructor func(frame frame.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error)

func TestUnconstrainedMotion(t *testing.T) {
	planners := []seededPlannerConstructor{
		NewRRTConnectMotionPlannerWithSeed,
		NewCBiRRTMotionPlannerWithSeed,
	}
	testCases := []struct {
		name   string
		config planConfig
	}{
		{"2D plan test", simple2DMap(t)},
		{"6D plan test", simpleUR5eMotion(t)},
		{"7D plan test", simpleXArmMotion(t)},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, planner := range planners {
				testPlanner(t, planner, &testCase.config)
			}
		})
	}
}

// TestConstrainedArmMotion tests a simple linear motion on a longer path, with a no-spill constraint.
func TestConstrainedArmMotion(t *testing.T) {
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	mp, err := NewCBiRRTMotionPlanner(m, nCPU/4, logger.Sugar())
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
		X:  -206,
		Y:  100,
		Z:  120,
		OZ: -1,
	}

	opt := NewDefaultPlannerOptions()
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

	path, err := mp.Plan(context.Background(), pos, home7, opt)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(path), test.ShouldNotEqual, 0)
}

func TestFixOvIncrement(t *testing.T) {
	pos1 := &commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,

		OZ: 0,
	}
	pos2 := &commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,
		OZ:    0,
	}
	// Increment, but we're not pointing at Z axis, so should do nothing
	pos2.OX = -0.1
	outpos := fixOvIncrement(pos2, pos1)
	test.That(t, outpos, test.ShouldResemble, pos2)

	// point at positive Z axis, decrement OX, should subtract 180
	pos1.OZ = 1
	pos2.OZ = 1
	pos1.OY = 0
	pos2.OY = 0
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos.Theta, test.ShouldEqual, -165)

	// Spatial translation is incremented, should do nothing
	pos2.X -= 0.1
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos, test.ShouldResemble, pos2)

	// Point at -Z, increment OY
	pos2.X += 0.1
	pos2.OX += 0.1
	pos1.OZ = -1
	pos2.OZ = -1
	pos2.OY = 0.1
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos.Theta, test.ShouldEqual, 105)

	// OX and OY are both incremented, should do nothing
	pos2.OX += 0.1
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos, test.ShouldResemble, pos2)
}

// simple2DMapConfig returns a planConfig with the following map
//		- start at (-9, 9) and end at (9, 9)
//      - bounds are from (-10, -10) to (10, 10)
//      - obstacle from (-4, 4) to (4, 10)
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
// ------------------------
func simple2DMap(t *testing.T) planConfig {
	// build model
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}
	physicalGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	model, err := frame.NewMobile2DFrame("mobile-base", limits, physicalGeometry)
	test.That(t, err, test.ShouldBeNil)

	// obstacles
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 6, 0}), r3.Vector{8, 8, 1})
	test.That(t, err, test.ShouldBeNil)

	return planConfig{
		NumTests:   1,
		Start:      frame.FloatsToInputs([]float64{-9., 9.}),
		Goal:       spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: 9, Y: 9, Z: 0})),
		RobotFrame: model,
		Obstacles:  []spatial.Geometry{box},
	}
}

// simpleArmMotion tests moving an xArm7
func simpleXArmMotion(t *testing.T) planConfig {
	xarm, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	return planConfig{
		NumTests:   1,
		Start:      home7,
		Goal:       &commonpb.Pose{X: 206, Y: 100, Z: 120, OZ: -1},
		RobotFrame: xarm,
	}
}

// simpleUR5eMotion tests a simple motion for a UR5e.
func simpleUR5eMotion(t *testing.T) planConfig {
	ur5e, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	return planConfig{
		NumTests:   1,
		Start:      home6,
		Goal:       &commonpb.Pose{X: -750, Y: -250, Z: 200, OX: -1},
		RobotFrame: ur5e,
	}
}

// testPlanner is a helper function that takes a planner and a planning query specified through a config object and tests that it
// returns a valid set of waypoints
func testPlanner(t *testing.T, planner seededPlannerConstructor, config *planConfig) {
	total := 0.
	allPaths := make([][][]frame.Input, config.NumTests)
	for i := 0; i < config.NumTests; i++ {
		// setup planner
		mp, err := planner(config.RobotFrame, nCPU/4, rand.New(rand.NewSource(int64(i))), logger.Sugar())
		test.That(t, err, test.ShouldBeNil)
		opt := NewDefaultPlannerOptions()
		toMap := func(geometries []spatial.Geometry) map[string]spatial.Geometry {
			geometryMap := make(map[string]spatial.Geometry, 0)
			for i, geometry := range geometries {
				geometryMap[strconv.Itoa(i)] = geometry
			}
			return geometryMap
		}
		opt.AddConstraint("collision", NewCollisionConstraint(config.RobotFrame, toMap(config.Obstacles), toMap(config.InteractionSpaces)))

		// plan
		path, err := mp.Plan(context.Background(), config.Goal, config.Start, opt)
		test.That(t, err, test.ShouldBeNil)

		// evaluate
		test.That(t, len(path), test.ShouldBeGreaterThanOrEqualTo, 2)
		distance := 0.
		for j := 0; j < len(path)-1; j++ {
			startPos, err := config.RobotFrame.Transform(path[j])
			test.That(t, err, test.ShouldBeNil)
			endPos, err := config.RobotFrame.Transform(path[j+1])
			test.That(t, err, test.ShouldBeNil)
			ok, _ := opt.constraintHandler.CheckConstraintPath(&ConstraintInput{
				StartPos:   startPos,
				EndPos:     endPos,
				StartInput: path[j],
				EndInput:   path[j+1],
				Frame:      config.RobotFrame,
			}, 1e-3)
			// test.That(t, ok, test.ShouldBeTrue)
			_ = ok
			distance += L2Distance(frame.InputsToFloats(path[j]), frame.InputsToFloats(path[j+1]))
		}

		// log
		fmt.Println("Test ", i+1, ":\t", distance)
		total += distance
		allPaths[i] = path
	}
	fmt.Print("Average:\t", total/float64(config.NumTests), "\n\n")

	// write output
	test.That(t, writeJSONFile(utils.ResolveFile("motionplan/output.test"), allPaths), test.ShouldBeNil)
}

func writeJSONFile(filename string, data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, bytes, 0o644); err != nil {
		return err
	}
	return nil
}
