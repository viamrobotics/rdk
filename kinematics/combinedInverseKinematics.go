package kinematics

import (
	"sync"

	"go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
)

// CombinedIK TODO
type CombinedIK struct {
	solvers []InverseKinematics
	Mdl     *Model
	ID      int
	logger  golog.Logger
}

// ReturnTest TODO
type ReturnTest struct {
	ID      int
	Success bool
	Result  *pb.JointPositions
}

// CreateCombinedIKSolver creates a combined parallel IK solver with the number of models given
// Must pass at least two models. Two will produce one jacobian IK solver, and all additional
// models will create nlopt solvers with different random seeds
func CreateCombinedIKSolver(models []*Model, logger golog.Logger) *CombinedIK {
	ik := &CombinedIK{}
	if len(models) < 2 {
		// Anything calling this should check core counts
		return nil
	}
	ik.Mdl = models[0]
	models[1].SetSeed(1)
	ik.solvers = append(ik.solvers, CreateNloptIKSolver(models[1], logger))
	for i := 2; i < len(models); i++ {
		models[i].SetSeed(int64(i * 1000))
		ik.solvers = append(ik.solvers, CreateNloptIKSolver(models[i], logger))
	}
	for i, solver := range ik.solvers {
		solver.SetID(i)
	}
	ik.logger = logger
	return ik
}

// SetID TODO
func (ik *CombinedIK) SetID(id int) {
	ik.ID = id
}

// GetID TODO
func (ik *CombinedIK) GetID() int {
	return ik.ID
}

// GetMdl TODO
func (ik *CombinedIK) GetMdl() *Model {
	return ik.Mdl
}

// Halt TODO
func (ik *CombinedIK) Halt() {
	for _, solver := range ik.solvers {
		solver.Halt()
	}
}

// GetSolvers TODO
func (ik *CombinedIK) GetSolvers() []InverseKinematics {
	return ik.solvers
}

func runSolver(solver InverseKinematics, c chan ReturnTest, noMoreSolutions <-chan struct{}, pos *pb.ArmPosition, seed *pb.JointPositions) {
	solved, result := solver.Solve(pos, seed)
	select {
	case c <- ReturnTest{solver.GetID(), solved, result}:
	case <-noMoreSolutions:
	}
}

// Solve TODO
func (ik *CombinedIK) Solve(pos *pb.ArmPosition, seed *pb.JointPositions) (bool, *pb.JointPositions) {
	ik.logger.Debugf("starting joint positions: %v", seed)
	ik.logger.Debugf("starting 6d position: %v", ComputePosition(ik.Mdl, seed))
	c := make(chan ReturnTest)

	noMoreSolutions := make(chan struct{})
	var activeSolvers sync.WaitGroup
	activeSolvers.Add(len(ik.solvers))
	for _, solver := range ik.solvers {
		thisSolver := solver
		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()
			runSolver(thisSolver, c, noMoreSolutions, pos, seed)
		})
	}

	returned := 0
	myRT := ReturnTest{-1, false, &pb.JointPositions{}}

	// Wait until either 1) we have a success or 2) all solvers have returned false
	for !myRT.Success && returned < len(ik.solvers) {
		myRT = <-c
		returned++
		if myRT.Success {
			ik.logger.Debugf("solved joint positions: %v", myRT.Result)
			ik.logger.Debugf("solved 6d position: %v", ComputePosition(ik.Mdl, myRT.Result))
		}
	}
	ik.Halt()
	close(noMoreSolutions)
	activeSolvers.Wait()
	return myRT.Success, myRT.Result
}
