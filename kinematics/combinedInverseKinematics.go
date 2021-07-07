package kinematics

import (
	"context"
	"sync"

	"go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
)

// CombinedIK TODO
type CombinedIK struct {
	solvers []InverseKinematics
	model   *Model
	logger  golog.Logger
}

// ReturnTest TODO
type ReturnTest struct {
	Success bool
	Result  *pb.JointPositions
}

// CreateCombinedIKSolver creates a combined parallel IK solver with the number of models given
// Must pass at least two models. Two will produce one jacobian IK solver, and all additional
// models will create nlopt solvers with different random seeds
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
	solved, result := solver.Solve(ctx, pos, seed)
	select {
	case c <- ReturnTest{solved, result}:
	case <-noMoreSolutions:
	}
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions.
func (ik *CombinedIK) Solve(ctx context.Context, pos *pb.ArmPosition, seed *pb.JointPositions) (bool, *pb.JointPositions) {
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
	myRT := ReturnTest{false, &pb.JointPositions{}}

	// Wait until either 1) we have a success or 2) all solvers have returned false
	for !myRT.Success && returned < len(ik.solvers) {
		myRT = <-c
		returned++
		if myRT.Success {
			ik.logger.Debugf("solved joint positions: %v", myRT.Result)
			ik.logger.Debugf("solved 6d position: %v", ComputePosition(ik.model, myRT.Result))
		}
	}
	cancel()
	close(noMoreSolutions)
	activeSolvers.Wait()
	return myRT.Success, myRT.Result
}

// Mdl returns the model associated with this IK.
func (ik *CombinedIK) Mdl() *Model {
	return ik.model
}
