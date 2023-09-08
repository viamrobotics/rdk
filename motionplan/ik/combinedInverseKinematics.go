//go:build !windows && cgo

package ik

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
)

// CombinedIK defines the fields necessary to run a combined solver.
type CombinedIK struct {
	solvers []InverseKinematics
	model   referenceframe.Frame
	logger  golog.Logger
}

// CreateCombinedIKSolver creates a combined parallel IK solver with a number of nlopt solvers equal to the nCPU
// passed in. Each will be given a different random seed. When asked to solve, all solvers will be run in parallel
// and the first valid found solution will be returned.
func CreateCombinedIKSolver(model referenceframe.Frame, logger golog.Logger, nCPU int, goalThreshold float64) (*CombinedIK, error) {
	ik := &CombinedIK{}
	ik.model = model
	if nCPU == 0 {
		nCPU = 1
	}
	for i := 1; i <= nCPU; i++ {
		nlopt, err := CreateNloptIKSolver(model, logger, -1, true)
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
func (ik *CombinedIK) Solve(ctx context.Context,
	c chan<- *Solution,
	seed []referenceframe.Input,
	m StateMetric,
	rseed int,
) error {
	var err error
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	errChan := make(chan error, len(ik.solvers))
	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()
	activeSolvers.Add(len(ik.solvers))

	for _, solver := range ik.solvers {
		rseed += 1500
		parseed := rseed
		thisSolver := solver

		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()

			errChan <- thisSolver.Solve(ctxWithCancel, c, seed, m, parseed)
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

// Frame returns the associated referenceframe.
func (ik *CombinedIK) Frame() referenceframe.Frame {
	return ik.model
}
