package kinematics

import (
	"context"
	"errors"
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
	for i := 1; i < nCPU; i++ {
		nlopt := CreateNloptIKSolver(model, logger)
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
	ik.logger.Debugf("starting 6d position: %v", ComputePosition(ik.model, seed))
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

	// Wait until either 1) we have a success or 2) all solvers have returned false
	for myRT.Err != nil && returned < len(ik.solvers) {
		myRT = <-c
		returned++
		if myRT.Err == nil {
			ik.logger.Debugf("solved joint positions: %v", myRT.Result)
			ik.logger.Debugf("solved 6d position: %v", ComputePosition(ik.model, myRT.Result))
		}
	}
	cancel()
	close(noMoreSolutions)
	activeSolvers.Wait()
	return myRT.Result, myRT.Err
}

// Mdl returns the model associated with this IK.
func (ik *CombinedIK) Mdl() *Model {
	return ik.model
}
