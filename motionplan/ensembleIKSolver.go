package motionplan

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ensembleIKSolver defines the fields necessary to run a combined solver.
type ensembleIKSolver struct {
	*ikSolver
	solvers []inverseKinematicsSolver
}

// newEnsembleIKSolver creates a combined parallel IK solver with a number of nlopt solvers equal to the nCPU
// passed in. Each will be given a different random seed. When asked to solve, all solvers will be run in parallel
// and the first valid found solution will be returned.
func newEnsembleIKSolver(model referenceframe.Frame, logger golog.Logger, opts *ikOptions) (*ensembleIKSolver, error) {
	ik := &ensembleIKSolver{ikSolver: &ikSolver{
		logger: logger,
		model:  model,
		opts:   opts,
	}}

	for i := 1; i <= opts.NumThreads; i++ {
		nlopt, err := newNLOptIKSolver(model, logger, opts)
		nlopt.id = i
		if err != nil {
			return nil, err
		}
		ik.solvers = append(ik.solvers, nlopt)
	}
	return ik, nil
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions. If unable to solve, the returned error will be non-nil.
func (ik *ensembleIKSolver) solve(ctx context.Context,
	c chan<- []referenceframe.Input,
	newGoal spatialmath.Pose,
	seed []referenceframe.Input,
	m Metric,
	rseed int,
) error {
	ik.logger.Debugf("starting inputs: %v", seed)
	startPos, err := ik.model.Transform(seed)
	if err != nil {
		return err
	}
	// This will adjust the goal position to make movements more intuitive when using incrementation near poles
	ik.logger.Debugf("starting pose: %v", spatialmath.PoseToProtobuf(startPos))
	ik.logger.Debugf("goal pose: %v", spatialmath.PoseToProtobuf(newGoal))

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
			errChan <- thisSolver.solve(ctxWithCancel, c, newGoal, seed, m, parseed)
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
			return ctx.Err()
		default:
		}

		err = <-errChan
		returned++
		if err != nil {
			collectedErrs = multierr.Combine(collectedErrs, err)
		}
	}
	return collectedErrs
}
