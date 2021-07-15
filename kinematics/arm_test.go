package kinematics

import (
	"context"
	"math/rand"
	"runtime"
	"testing"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

const toSolve = 100

var (
	home = arm.JointPositionsFromRadians([]float64{0, 0, 0, 0, 0, 0})
	nCPU = runtime.NumCPU()
	seed = rand.New(rand.NewSource(1))
)

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateCombinedIKSolver(m, logger, nCPU)

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  -46,
		Y:  -133,
		Z:  372,
		OX: 1.79,
		OY: -1.32,
		OZ: -1.11,
	}
	solution, err := ik.Solve(context.Background(), pos, home)
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
	_, err = ik.Solve(context.Background(), pos, solution)
	test.That(t, err, test.ShouldBeNil)
}

func BenchCombinedIKinematics(t *testing.B) {
	logger := golog.NewDevelopmentLogger("combinedBenchmark")

	m, err := ParseJSONFile(utils.ResolveFile("robots/eva/eva_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateCombinedIKSolver(m, logger, nCPU)

	// Test we are able to solve random valid positions from other random valid positions
	// Used for benchmarking solve rate
	solvedCnt := 0
	for i := 0; i < toSolve; i++ {
		randJointPos := arm.JointPositionsFromRadians(m.GenerateRandomJointPositions(seed))
		randPos, err := ComputePosition(m, randJointPos)
		test.That(t, err, test.ShouldBeNil)
		_, err = ik.Solve(context.Background(), randPos, home)
		if err == nil {
			solvedCnt++
		}
	}
	logger.Debug("combined solved: ", solvedCnt)
}

func TestUR5NloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateCombinedIKSolver(m, logger, nCPU)

	goalJP := arm.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal, err := ComputePosition(m, goalJP)
	test.That(t, err, test.ShouldBeNil)
	_, err = ik.Solve(context.Background(), goal, home)
	test.That(t, err, test.ShouldBeNil)
}

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := ParseJSONFile(utils.ResolveFile("robots/varm/v1_test.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateCombinedIKSolver(m, logger, nCPU)

	// Test inability to arrive at another position due to orientation
	pos := &pb.ArmPosition{
		X:  -46,
		Y:  0,
		Z:  372,
		OX: -1.78,
		OY: -3.3,
		OZ: -1.11,
	}
	_, err = ik.Solve(context.Background(), pos, home)
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	m, err = ParseJSONFile(utils.ResolveFile("robots/varm/v1.json"))
	test.That(t, err, test.ShouldBeNil)
	ik = CreateCombinedIKSolver(m, logger, nCPU)

	_, err = ik.Solve(context.Background(), pos, home)
	test.That(t, err, test.ShouldBeNil)
}
