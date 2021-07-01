package kinematics

import (
	"context"
	"runtime"
	"testing"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

const toSolve = 100

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	ctx := context.Background()
	dummy := inject.Arm{}

	jPos := arm.JointPositionsFromRadians([]float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379})

	dummy.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return jPos, nil
	}
	dummy.MoveToJointPositionsFunc = func(ctx context.Context, joints *pb.JointPositions) error {
		jPos = joints
		return nil
	}

	logger := golog.NewTestLogger(t)
	nCPU := runtime.NumCPU()
	wxArm, err := NewArmJSONFile(&dummy, utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  -46,
		Y:  -133,
		Z:  372,
		OX: 1.79,
		OY: -1.32,
		OZ: -1.11,
	}
	err = wxArm.MoveToPosition(ctx, pos)
	test.That(t, err, test.ShouldBeNil)

	// Test moving forward 20 in X direction from previous position
	pos = &pb.ArmPosition{
		X:  -66,
		Y:  -133,
		Z:  372,
		OX: -178.88747811107424,
		OY: -33.160094626838045,
		OZ: -111.02282693533935,
	}
	err = wxArm.MoveToPosition(ctx, pos)
	test.That(t, err, test.ShouldBeNil)
}

func BenchCombinedIKinematics(t *testing.B) {
	ctx := context.Background()
	logger := golog.NewDevelopmentLogger("combinedBenchmark")
	nCPU := runtime.NumCPU()

	dummy := inject.Arm{}
	jPos := arm.JointPositionsFromRadians([]float64{0, 0, 0, 0, 0, 0})
	dummy.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return jPos, nil
	}
	dummy.MoveToJointPositionsFunc = func(ctx context.Context, joints *pb.JointPositions) error {
		jPos = joints
		return nil
	}

	eva, err := NewArmJSONFile(&dummy, utils.ResolveFile("robots/eva/eva_kinematics.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	// Used for benchmarking solve rate
	solved := 0
	for i := 0; i < toSolve; i++ {
		randJointPos := arm.JointPositionsFromRadians(eva.Model.RandomJointPositions())
		randPos := ComputePosition(eva.Model, randJointPos)
		dummy.MoveToJointPositions(ctx, arm.JointPositionsFromRadians([]float64{0, 0, 0, 0, 0, 0}))
		err = eva.MoveToPosition(ctx, randPos)
		if err == nil {
			solved++
		}
	}
	logger.Debug("combined solved: ", solved)
}

func TestUR5NloptIKinematics(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	dummy := inject.Arm{}
	jPos := arm.JointPositionsFromRadians([]float64{0.01, -2.0, 1.98, -1.771, -1.754, -0.4})
	dummy.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return jPos, nil
	}
	dummy.MoveToJointPositionsFunc = func(ctx context.Context, joints *pb.JointPositions) error {
		jPos = joints
		return nil
	}
	ur5e, err := NewArmJSONFile(&dummy, utils.ResolveFile("robots/universalrobots/ur5e.json"), 2, logger)
	test.That(t, err, test.ShouldBeNil)

	goalJP := arm.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal := ComputePosition(ur5e.Model, goalJP)
	err = ur5e.MoveToPosition(ctx, goal)
	test.That(t, err, test.ShouldBeNil)
}

func TestIKTolerances(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	nCPU := runtime.NumCPU()

	dummy := inject.Arm{}
	jPos := arm.JointPositionsFromRadians([]float64{0, 5})
	dummy.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return jPos, nil
	}
	dummy.MoveToJointPositionsFunc = func(ctx context.Context, joints *pb.JointPositions) error {
		jPos = joints
		return nil
	}

	v1Arm, err := NewArmJSONFile(&dummy, utils.ResolveFile("robots/varm/v1_test.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	pos := &pb.ArmPosition{
		X:  -46,
		Y:  0,
		Z:  372,
		OX: -1.78,
		OY: -3.3,
		OZ: -1.11,
	}
	err = v1Arm.MoveToPosition(ctx, pos)

	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	v1Arm, err = NewArmJSONFile(&dummy, utils.ResolveFile("robots/varm/v1.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)
	err = v1Arm.MoveToPosition(ctx, pos)

	test.That(t, err, test.ShouldBeNil)
}
