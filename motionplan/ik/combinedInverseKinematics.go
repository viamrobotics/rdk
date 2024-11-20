//go:build !windows && !no_cgo

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

// combinedIK defines the fields necessary to run a combined solver.
type combinedIK struct {
	solvers []Solver
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
) (Solver, error) {
	ik := &combinedIK{}
	ik.limits = limits
	if nCPU == 0 {
		nCPU = 1
	}
	for i := 1; i <= nCPU; i++ {
		solver, err := CreateNloptSolver(ik.limits, logger, -1, true, true)
		nlopt := solver.(*nloptIK)
		nlopt.id = i
		if err != nil {
			return nil, err
		}
		ik.solvers = append(ik.solvers, nlopt)
	}
	ik.logger = logger
	return ik, nil
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions. If unable to solve, the returned error will be non-nil.
func (ik *combinedIK) Solve(ctx context.Context,
	c chan<- *Solution,
	seed []float64,
	m func([]float64) float64,
	rseed int,
) error {
	var err error
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	//nolint: gosec
	randSeed := rand.New(rand.NewSource(int64(rseed)))

	errChan := make(chan error, len(ik.solvers))
	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()
	activeSolvers.Add(len(ik.solvers))

	lowerBound, upperBound := limitsToArrays(ik.limits)

	for i, solver := range ik.solvers {
		rseed += 1500
		parseed := rseed
		thisSolver := solver
		seedFloats := seed
		if i > 0 {
			seedFloats = generateRandomPositions(randSeed, lowerBound, upperBound)
		}

		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()

			errChan <- thisSolver.Solve(ctxWithCancel, c, seedFloats, m, parseed)
		})
	}

	returned := 0
	done := false

	var collectedErrs error

	// Wait until either 1) we have a success or 2) all solvers have returned false
	// Multiple selects are necessary in the case where we get a ctx.Done() while there is also an error waiting
	for !done {
		select {
		case <-ctx.Done():
			activeSolvers.Wait()
			return ctx.Err()
		default:
		}

		select {
		case err = <-errChan:
			returned++
			if err != nil {
				collectedErrs = multierr.Combine(collectedErrs, err)
			}
		default:
			if returned == len(ik.solvers) {
				done = true
			}
		}
	}
	cancel()
	for returned < len(ik.solvers) {
		// Collect return errors from all solvers
		select {
		case <-ctx.Done():
			activeSolvers.Wait()
			return ctx.Err()
		default:
		}

		err = <-errChan
		returned++
		if err != nil {
			collectedErrs = multierr.Combine(collectedErrs, err)
		}
	}
	activeSolvers.Wait()
	return collectedErrs
}

// DoF returns the DoF of the solver.
func (ik *combinedIK) DoF() []referenceframe.Limit {
	return ik.limits
}
