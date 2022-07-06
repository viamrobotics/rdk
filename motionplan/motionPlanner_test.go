package motionplan

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestSimple2DMotion(t *testing.T) {
	// Test Map:
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

	// setup problem parameters
	start := frame.FloatsToInputs([]float64{-9., 9.})
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: 9, Y: 9, Z: 0}))
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build obstacles
	obstacles := map[string]spatial.Geometry{}
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 6, 0}), r3.Vector{8, 8, 1})
	test.That(t, err, test.ShouldBeNil)
	obstacles["box"] = box

	// build model
	physicalGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	model, err := frame.NewMobile2DFrame("mobile-base", limits, physicalGeometry)
	test.That(t, err, test.ShouldBeNil)

	// plan
	cbert, err := NewCBiRRTMotionPlanner(model, 1, logger)
	test.That(t, err, test.ShouldBeNil)
	opt := NewDefaultPlannerOptions()
	constraint := NewCollisionConstraint(model, obstacles, map[string]spatial.Geometry{})
	test.That(t, err, test.ShouldBeNil)
	opt.AddConstraint("collision", constraint)
	waypoints, err := cbert.Plan(context.Background(), goal, start, opt)
	test.That(t, err, test.ShouldBeNil)

	obstacle := obstacles["box"]
	workspace, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{20, 20, 1})
	test.That(t, err, test.ShouldBeNil)
	for _, waypoint := range waypoints {
		pt := r3.Vector{waypoint[0].Value, waypoint[1].Value, 0}
		collides, err := obstacle.CollidesWith(spatial.NewPoint(pt))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		inWorkspace, err := workspace.CollidesWith(spatial.NewPoint(pt))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, inWorkspace, test.ShouldBeTrue)
		logger.Debugf("%f\t%f\n", pt.X, pt.Y)
	}
}

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	home6 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
)

// This should test a simple linear motion for Arms.
func TestSimpleArmMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	mp, err := NewCBiRRTMotionPlanner(m, nCPU/4, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
		X:  206,
		Y:  100,
		Z:  120,
		OZ: -1,
	}
	path, err := mp.Plan(context.Background(), pos, home7, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(path), test.ShouldNotEqual, 0)
}

// This should test a simple linear motion on a longer path, with a no-spill constraint.
func TestComplexArmMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	mp, err := NewCBiRRTMotionPlanner(m, nCPU/4, logger)
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

// This should test a simple linear motion for a UR5e.
func TestSimpleMotionUR5e(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mp, err := NewCBiRRTMotionPlanner(m, nCPU/4, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
		X:  -750,
		Y:  -250,
		Z:  200,
		OX: -1,
	}
	path, err := mp.Plan(context.Background(), pos, home6, nil)
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
