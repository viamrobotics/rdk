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
		OX: 1.78,
		OY: -3.3,
		OZ: -1.11,
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
	_, err = ik.Solve(context.Background(), pos, arm.JointPositionsFromRadians([]float64{0, 0}))
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	m, err = ParseJSONFile(utils.ResolveFile("robots/varm/v1.json"))
	test.That(t, err, test.ShouldBeNil)
	ik = CreateCombinedIKSolver(m, logger, nCPU)

	_, err = ik.Solve(context.Background(), pos, arm.JointPositionsFromRadians([]float64{0, 0}))
	test.That(t, err, test.ShouldBeNil)
}

func TestSVAvsDH(t *testing.T) {
	mSVA, err := ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"))
	test.That(t, err, test.ShouldBeNil)
	mDH, err := ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e_DH.json"))
	test.That(t, err, test.ShouldBeNil)

	numTests := 10000

	seed := rand.New(rand.NewSource(23))
	for i := 0; i < numTests; i++ {
		joints := arm.JointPositionsFromRadians(mSVA.GenerateRandomJointPositions(seed))

		posSVA, err := ComputePosition(mSVA, joints)
		test.That(t, err, test.ShouldBeNil)
		posDH, err := ComputePosition(mDH, joints)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, posSVA.X, test.ShouldAlmostEqual, posDH.X, .01)
		test.That(t, posSVA.Y, test.ShouldAlmostEqual, posDH.Y, .01)
		test.That(t, posSVA.Z, test.ShouldAlmostEqual, posDH.Z, .01)

		test.That(t, posSVA.OX, test.ShouldAlmostEqual, posDH.OX, .01)
		test.That(t, posSVA.OY, test.ShouldAlmostEqual, posDH.OY, .01)
		test.That(t, posSVA.OZ, test.ShouldAlmostEqual, posDH.OZ, .01)
		test.That(t, posSVA.Theta, test.ShouldAlmostEqual, posDH.Theta, .01)
	}
}
