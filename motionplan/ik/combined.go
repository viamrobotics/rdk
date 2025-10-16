package ik

import (
	"context"
	"math/rand"
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
	limits  []referenceframe.Limit
}

// CreateCombinedIKSolver creates a combined parallel IK solver that operates on a frame with a number of nlopt solvers equal to the
// nCPU passed in. Each will be given a different random seed. When asked to solve, all solvers will be run in parallel
// and the first valid found solution will be returned.
func CreateCombinedIKSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	nCPU int,
	goalThreshold float64,
) (*CombinedIK, error) {
	ik := &CombinedIK{}
	ik.limits = limits
	nCPU = max(nCPU, 2)

	for i := 1; i <= nCPU; i++ {
		nloptSolver, err := CreateNloptSolver(ik.limits, logger, -1, true, true)
		if err != nil {
			return nil, err
		}
		ik.solvers = append(ik.solvers, nloptSolver)
	}
	ik.logger = logger
	return ik, nil
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions. If unable to solve, the returned error will be non-nil.
func (ik *CombinedIK) Solve(ctx context.Context,
	retChan chan<- *Solution,
	seed []float64,
	travelPercent []float64,
	m CostFunc,
	rseed int,
) (int, error) {
	randSeed := rand.New(rand.NewSource(int64(rseed))) //nolint: gosec

	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()

	lowerBound, upperBound := limitsToArrays(ik.limits)

	var solveErrors error
	totalSolutionsFound := 0
	var solveResultLock sync.Mutex

	for i, solver := range ik.solvers {
		rseed += 1500
		parseed := rseed
		thisSolver := solver
		seedFloats := seed

		var myTravelPercent []float64
		if i <= len(ik.solvers)/3 {
			// TODO: this is probably too conservative
			for _, p := range travelPercent {
				myTravelPercent = append(myTravelPercent, max(.1, p))
			}
		} else if i < (2 * len(ik.solvers) / 3) {
			myTravelPercent = travelPercent
		} else {
			seedFloats = generateRandomPositions(randSeed, lowerBound, upperBound)
		}

		activeSolvers.Add(1)
		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()

			n, err := thisSolver.Solve(ctx, retChan, seedFloats, myTravelPercent, m, parseed)

			solveResultLock.Lock()
			defer solveResultLock.Unlock()
			totalSolutionsFound += n
			solveErrors = multierr.Combine(solveErrors, err)
		})
	}

	activeSolvers.Wait()
	return totalSolutionsFound, solveErrors
}

// DoF returns the DoF of the solver.
func (ik *CombinedIK) DoF() []referenceframe.Limit {
	return ik.limits
}
