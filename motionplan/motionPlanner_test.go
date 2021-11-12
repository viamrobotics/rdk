package motionplan

import (
	"context"
	"fmt"

	"runtime"
	"testing"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	home6 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	nCPU  = runtime.NumCPU()
)

// This should test a simple linear motion
func TestSimpleMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	mp, err := NewCBiRRTMotionPlanner_petertest(m, logger, 4)
	test.That(t, err, test.ShouldBeNil)
	//~ mp.AddConstraint("orientation", NewPoseConstraint())

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  206,
		Y:  100,
		Z:  120,
		OZ: -1,
	}
	path, err := mp.Plan(context.Background(), pos, home7)
	test.That(t, err, test.ShouldBeNil)

	fmt.Println(path)
}

// This should test a simple linear motion
func TestSimpleMotionUR5(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"))
	test.That(t, err, test.ShouldBeNil)
	mp, err := NewCBiRRTMotionPlanner(m, logger, 4)
	test.That(t, err, test.ShouldBeNil)

	mp.RemoveConstraint("orientation")
	mp.RemoveConstraint("obstacle")

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  -750,
		Y:  -250,
		Z:  200,
		OZ: -1,
	}
	path, err := mp.Plan(context.Background(), pos, home6)
	test.That(t, err, test.ShouldBeNil)

	fmt.Println(path)
}
