package motionplan

import (
	"context"
	"fmt"

	//~ "math"

	"runtime"
	"testing"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

var (
	home = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	nCPU = runtime.NumCPU()
)

// This should test a simple linear motion
func TestSimpleMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik, err := kinematics.CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	mp := &linearMotionPlanner{solver: ik, frame: m}

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  250,
		Y:  0,
		Z:  200,
		OZ: -1,
	}
	solution, err := mp.Plan(context.Background(), spatial.NewPoseFromArmPos(pos), home)
	test.That(t, err, test.ShouldBeNil)

	fmt.Println(solution)
}
