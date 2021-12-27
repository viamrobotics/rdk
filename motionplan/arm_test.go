package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"runtime"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const toSolve = 100

var (
	home = referenceframe.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	nCPU = runtime.NumCPU() / 4
	seed = rand.New(rand.NewSource(1))
)

// This should test all of the kinematics functions.
func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
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
	solution, err := solveTest(context.Background(), ik, pos, home)
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
	_, err = solveTest(context.Background(), ik, pos, solution[0])
	test.That(t, err, test.ShouldBeNil)
}

func BenchCombinedIKinematics(b *testing.B) {
	b.Helper()
	logger := golog.NewDevelopmentLogger("combinedBenchmark")

	m, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/eva/eva_json"), "")
	test.That(b, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(b, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	// Used for benchmarking solve rate
	solvedCnt := 0
	for i := 0; i < toSolve; i++ {
		randJointPos := referenceframe.GenerateRandomJointPositions(m, seed)
		randPos, err := ComputePosition(m, referenceframe.JointPositionsFromRadians(randJointPos))
		test.That(b, err, test.ShouldBeNil)
		solution, err := solveTest(context.Background(), ik, randPos, home)
		test.That(b, solution, test.ShouldNotBeNil)
		test.That(b, checkGoodJointDelta([]float64{0, 0, 0, 0, 0, 0}, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		if err == nil {
			solvedCnt++
		}
	}
	logger.Debug("combined solved: ", solvedCnt)
}

func TestUR5NloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	goalJP := referenceframe.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal, err := ComputePosition(m, goalJP)
	test.That(t, err, test.ShouldBeNil)
	_, err = solveTest(context.Background(), ik, goal, home)
	test.That(t, err, test.ShouldBeNil)
}

func TestSVAvsDH(t *testing.T) {
	mSVA, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mDH, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e_DH.json"), "")
	test.That(t, err, test.ShouldBeNil)

	numTests := 10000

	seed := rand.New(rand.NewSource(23))
	for i := 0; i < numTests; i++ {
		joints := referenceframe.JointPositionsFromRadians(referenceframe.GenerateRandomJointPositions(mSVA, seed))

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

func BenchNloptSwing(b *testing.B) {
	b.Helper()
	logger := golog.NewDevelopmentLogger("testSwing")
	m, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_json"), "")
	test.That(b, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(b, err, test.ShouldBeNil)

	// Test we are able to solve incremental changes without large joint swings
	for i := 0; i < toSolve; i++ {
		origRadians := referenceframe.GenerateRandomJointPositions(m, seed)
		randJointPos := referenceframe.FloatsToInputs(origRadians)
		randPos, err := ComputePosition(m, referenceframe.JointPositionsFromRadians(origRadians))
		test.That(b, err, test.ShouldBeNil)
		randPos.X += 10
		solution, err := solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		}

		randPos.Y += 10
		solution, err = solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		}

		randPos.Z += 10
		solution, err = solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		}

		randPos.OX += 0.1
		solution, err = solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		}

		randPos.OY += 0.1
		solution, err = solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		}

		randPos.OZ += 0.1
		solution, err = solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
		}

		randPos.Theta += 45
		solution, err = solveTest(context.Background(), ik, randPos, randJointPos)
		if err == nil {
			test.That(b, checkGoodJointDelta(origRadians, referenceframe.InputsToFloats(solution[0])), test.ShouldBeTrue)
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

func TestCombinedCPUs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_test.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, runtime.NumCPU()/400000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(ik.solvers), test.ShouldEqual, 1)
}

func solveTest(
	ctx context.Context,
	solver InverseKinematics,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
) ([][]referenceframe.Input, error) {
	goalPos := spatial.NewPoseFromProtobuf(goal)

	solutionGen := make(chan []referenceframe.Input)
	ikErr := make(chan error)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn the IK solver to generate solutions until done
	go func() {
		defer close(ikErr)
		ikErr <- solver.Solve(ctxWithCancel, solutionGen, goalPos, seed)
	}()

	var solutions [][]referenceframe.Input

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		select {
		case step := <-solutionGen:
			solutions = append(solutions, step)
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			break IK
		default:
		}
	}
	cancel()
	if len(solutions) == 0 {
		return nil, errors.New("unable to solve for position")
	}

	return solutions, nil
}
