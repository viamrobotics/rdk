package kinematics

import (
	"context"
	"errors"
	"math"
	"sync"

	"go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
)

// CombinedIK defines the fields necessary to run a combined solver.
type CombinedIK struct {
	solvers []InverseKinematics
	model   *Model
	logger  golog.Logger
}

// ReturnTest is the struct used to communicate over a channel with the child parallel solvers.
type ReturnTest struct {
	Err    error
	Result *pb.JointPositions
}

// CreateCombinedIKSolver creates a combined parallel IK solver with a number of nlopt solvers equal to the nCPU
// passed in. Each will be given a different random seed. When asked to solve, all solvers will be run in parallel
// and the first valid found solution will be returned.
func CreateCombinedIKSolver(model *Model, logger golog.Logger, nCPU int) *CombinedIK {
	ik := &CombinedIK{}
	ik.model = model
	for i := 1; i <= nCPU; i++ {
		nlopt := CreateNloptIKSolver(model, logger, i)
		nlopt.SetSeed(int64(i * 1000))
		ik.solvers = append(ik.solvers, nlopt)
	}
	ik.logger = logger
	return ik
}

func runSolver(ctx context.Context, solver InverseKinematics, c chan ReturnTest, noMoreSolutions <-chan struct{}, pos *pb.ArmPosition, seed *pb.JointPositions) {
	result, err := solver.Solve(ctx, pos, seed)
	select {
	case c <- ReturnTest{err, result}:
	case <-noMoreSolutions:
	}
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions. If unable to solve, the returned error will be non-nil
func (ik *CombinedIK) Solve(ctx context.Context, pos *pb.ArmPosition, seed *pb.JointPositions) (*pb.JointPositions, error) {
	ik.logger.Debugf("starting joint positions: %v", seed)
	startPos, err := ComputePosition(ik.model, seed)
	ik.logger.Debugf("starting 6d position: %v %v", startPos, err)
	ik.logger.Debugf("goal 6d position: %v", pos)

	// This will adjust the goal position to make movements more intuitive when using incrementation near poles
	pos = fixOvIncrement(pos, startPos)
	ik.logger.Debugf("postfix goal 6d position: %v", pos)
	c := make(chan ReturnTest)
	ctxWithCancel, cancel := context.WithCancel(ctx)

	noMoreSolutions := make(chan struct{})
	var activeSolvers sync.WaitGroup
	activeSolvers.Add(len(ik.solvers))
	for _, solver := range ik.solvers {
		thisSolver := solver
		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()
			runSolver(ctxWithCancel, thisSolver, c, noMoreSolutions, pos, seed)
		})
	}

	returned := 0
	myRT := ReturnTest{errors.New("could not solve for position"), &pb.JointPositions{}}

	var solutions []*pb.JointPositions

	found := false

	// Wait until either 1) we have a success or 2) all solvers have returned false
	for !found && returned < len(ik.solvers) {
		myRT = <-c
		returned++
		if myRT.Err == nil {

			dist := calcSwingPct(seed, myRT.Result, ik.model)
			// Since distances are squared, a perfect "halfway" will have a dist ~0.25. Better than 0.5 is good enough.
			if dist < 0.5 {
				found = true
				solutions = []*pb.JointPositions{myRT.Result}
			} else {
				solutions = append(solutions, myRT.Result)
			}
		}
	}
	cancel()
	close(noMoreSolutions)
	activeSolvers.Wait()
	if len(solutions) > 0 {
		myRT.Result = bestSolution(seed, solutions, ik.model)
		myRT.Err = nil
		ik.logger.Debugf("solved joint positions: %v", myRT.Result)
		solvePos, err := ComputePosition(ik.model, myRT.Result)
		ik.logger.Debugf("solved 6d position: %v %v", solvePos, err)
	}
	return myRT.Result, myRT.Err
}

// Mdl returns the model associated with this IK.
func (ik *CombinedIK) Mdl() *Model {
	return ik.model
}

// fixOvIncrement will detect whether the given goal position is a precise orientation increment of the current
// position, in which case it will detect whether we are leaving a pole. If we are an OV increment and leaving a pole,
// then Theta will be adjusted to give an expected smooth movement. The adjusted goal will be returned. Otherwise the
// original goal is returned.
// Rationale: if clicking the increment buttons in the interface, the user likely wants the most intuitive motion
// posible. If setting values manually, the user likely wants exactly what they requested.
func fixOvIncrement(pos, seed *pb.ArmPosition) *pb.ArmPosition {
	epsilon := 0.0001
	// Nothing to do for spatial translations or theta increments
	if pos.X != seed.X || pos.Y != seed.Y || pos.Z != seed.Z || pos.Theta != seed.Theta {
		return pos
	}
	// Check if seed is pointing directly at pole
	if 1-math.Abs(seed.OZ) > epsilon || pos.OZ != seed.OZ {
		return pos
	}

	// we only care about negative xInc
	xInc := pos.OX - seed.OX
	yInc := math.Abs(pos.OY - seed.OY)
	adj := 0.0
	if pos.OX == seed.OX {
		// no OX movement
		if yInc != 0.1 && yInc != 0.01 {
			// nonstandard increment
			return pos
		}
		// If wanting to point towards +Y and OZ<0, add 90 to theta, otherwise subtract 90
		if pos.OY-seed.OY > 0 {
			adj = 90
		} else {
			adj = -90
		}
	} else {
		if (xInc != -0.1 && xInc != -0.01) || pos.OY != seed.OY {
			return pos
		}
		// If wanting to point towards -X, increment by 180. Values over 180 or under -180 will be automatically wrapped
		adj = 180
	}
	if pos.OZ > 0 {
		adj *= -1
	}

	return &pb.ArmPosition{
		X:     pos.X,
		Y:     pos.Y,
		Z:     pos.Z,
		Theta: pos.Theta + adj,
		OX:    pos.OX,
		OY:    pos.OY,
		OZ:    pos.OZ,
	}
}
