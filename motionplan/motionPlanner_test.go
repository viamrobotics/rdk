package motionplan

import (
	"context"
	"testing"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	home6 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
)

// This should test a simple linear motion
func TestSimpleMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	mp, err := NewCBiRRTMotionPlanner(m, nCPU/4, logger)
	test.That(t, err, test.ShouldBeNil)
	//~ mp.AddConstraint("orientation", NewPoseConstraint())

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
		X:  206,
		Y:  100,
		Z:  120,
		OZ: -1,
	}
	path, err := mp.Plan(context.Background(), pos, home7)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(path), test.ShouldNotEqual, 0)
}

// This should test a simple linear motion on a longer path, with a no-spill constraint
func TestComplexMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")
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
	mp.SetOptions(opt)

	path, err := mp.Plan(context.Background(), pos, home7)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(path), test.ShouldNotEqual, 0)
}

// This should test a simple linear motion
func TestSimpleMotionUR5(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mp, err := NewCBiRRTMotionPlanner(m, nCPU/4, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
		X:  -750,
		Y:  -250,
		Z:  200,
		OZ: -1,
	}
	path, err := mp.Plan(context.Background(), pos, home6)
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
