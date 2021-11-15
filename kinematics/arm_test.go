package kinematics

import (
	"context"
	"math"
	"math/rand"
	"runtime"
	"testing"

	commonpb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

const toSolve = 100

var (
	home = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	nCPU = runtime.NumCPU()
	seed = rand.New(rand.NewSource(1))
)

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
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
	pos = &commonpb.Pose{
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

	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/eva/eva_json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	// Used for benchmarking solve rate
	solvedCnt := 0
	for i := 0; i < toSolve; i++ {
		randJointPos := m.GenerateRandomJointPositions(seed)
		randPos, err := ComputePosition(m, frame.JointPositionsFromRadians(randJointPos))
		test.That(t, err, test.ShouldBeNil)
		solution, err := ik.Solve(context.Background(), randPos, home)
		test.That(t, solution, test.ShouldNotBeNil)
		test.That(t, checkGoodJointDelta([]float64{0, 0, 0, 0, 0, 0}, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		if err == nil {
			solvedCnt++
		}
	}
	logger.Debug("combined solved: ", solvedCnt)
}

func TestUR5NloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	goalJP := frame.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal, err := ComputePosition(m, goalJP)
	test.That(t, err, test.ShouldBeNil)
	_, err = ik.Solve(context.Background(), goal, home)
	test.That(t, err, test.ShouldBeNil)
}

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/varm/v1_test.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	pos := &commonpb.Pose{
		X:  -46,
		Y:  0,
		Z:  372,
		OX: -1.78,
		OY: -3.3,
		OZ: -1.11,
	}
	_, err = ik.Solve(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	m, err = frame.ParseJSONFile(utils.ResolveFile("robots/varm/v1.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err = CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)
	ik.SetSolveWeights(m.SolveWeights)

	_, err = ik.Solve(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldBeNil)
}

func TestSVAvsDH(t *testing.T) {
	mSVA, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mDH, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e_DH.json"), "")
	test.That(t, err, test.ShouldBeNil)

	numTests := 10000

	seed := rand.New(rand.NewSource(23))
	for i := 0; i < numTests; i++ {
		joints := frame.JointPositionsFromRadians(mSVA.GenerateRandomJointPositions(seed))

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

func BenchNloptSwing(t *testing.B) {
	logger := golog.NewDevelopmentLogger("testSwing")
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve incremental changes without large joint swings
	for i := 0; i < toSolve; i++ {
		origRadians := m.GenerateRandomJointPositions(seed)
		randJointPos := frame.FloatsToInputs(origRadians)
		randPos, err := ComputePosition(m, frame.JointPositionsFromRadians(origRadians))
		test.That(t, err, test.ShouldBeNil)
		randPos.X += 10
		solution, err := ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}

		randPos.Y += 10
		solution, err = ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}

		randPos.Z += 10
		solution, err = ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}

		randPos.OX += 0.1
		solution, err = ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}

		randPos.OY += 0.1
		solution, err = ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}

		randPos.OZ += 0.1
		solution, err = ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}

		randPos.Theta += 45
		solution, err = ik.Solve(context.Background(), randPos, randJointPos)
		if err == nil {
			test.That(t, checkGoodJointDelta(origRadians, frame.InputsToFloats(solution)), test.ShouldBeTrue)
		}
	}
}

func checkGoodJointDelta(orig, solution []float64) bool {
	for i, angle := range solution {
		if i < len(solution)-3 {
			if math.Abs(angle-orig[i]) > 2.8 {
				return false
			}
		}
	}
	return true
}
