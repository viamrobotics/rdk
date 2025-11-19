package ik

import (
	"context"
	"sync"

	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// CombinedIK defines the fields necessary to run a combined solver.
type CombinedIK struct {
	solvers []*NloptIK
	logger  logging.Logger
}

// CreateCombinedIKSolver creates a combined parallel IK solver that operates on a frame with a number of nlopt solvers equal to the
// nCPU passed in. Each will be given a different random seed. When asked to solve, all solvers will be run in parallel
// and the first valid found solution will be returned.
func CreateCombinedIKSolver(
	logger logging.Logger,
	nCPU int,
	goalThreshold float64,
) (*CombinedIK, error) {
	ik := &CombinedIK{
		logger: logger,
	}

	logger.Infof("CreateCombinedIKSolver nCPU: %d", nCPU)
	for i := 1; i <= nCPU; i++ {
		nloptSolver, err := CreateNloptSolver(logger, -1, true, true)
		if err != nil {
			return nil, err
		}
		ik.solvers = append(ik.solvers, nloptSolver)
	}
	return ik, nil
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions. If unable to solve, the returned error will be non-nil.
func (ik *CombinedIK) Solve(ctx context.Context,
	retChan chan<- *Solution,
	seeds [][]float64,
	limits [][]referenceframe.Limit,
	costFunc CostFunc,
	rseed int,
) (int, []SeedSolveMetaData, error) {
	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()

	var solveErrors error
	totalSolutionsFound := 0
	metas := []SeedSolveMetaData{}
	var solveResultLock sync.Mutex

	for _, solver := range ik.solvers {
		thisSolver := solver
		myseed := rseed
		rseed++

		activeSolvers.Add(1)
		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()

			n, m, err := thisSolver.Solve(ctx, retChan, seeds, limits, costFunc, myseed)

			solveResultLock.Lock()
			defer solveResultLock.Unlock()
			totalSolutionsFound += n
			solveErrors = multierr.Combine(solveErrors, err)
			if len(metas) == 0 {
				metas = m
			} else {
				for idx, mm := range m {
					metas[idx].Attempts += mm.Attempts
					metas[idx].Valid += mm.Valid
					metas[idx].Errors += mm.Errors
				}
			}
		})
	}

	activeSolvers.Wait()

	return totalSolutionsFound, metas, solveErrors
}
