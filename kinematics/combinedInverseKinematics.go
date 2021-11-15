package kinematics

import (
	"context"
	"sync"

	"go.viam.com/utils"

	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// CombinedIK defines the fields necessary to run a combined solver.
type CombinedIK struct {
	solvers []InverseKinematics
	model   frame.Frame
	logger  golog.Logger
}

// CreateCombinedIKSolver creates a combined parallel IK solver with a number of nlopt solvers equal to the nCPU
// passed in. Each will be given a different random seed. When asked to solve, all solvers will be run in parallel
// and the first valid found solution will be returned.
func CreateCombinedIKSolver(model frame.Frame, logger golog.Logger, nCPU int) (*CombinedIK, error) {
	ik := &CombinedIK{}
	ik.model = model
	for i := 1; i <= nCPU; i++ {
		nlopt, err := CreateNloptIKSolver(model, logger)
		nlopt.id = i
		if err != nil {
			return nil, err
		}
		nlopt.SetSeed(int64(i * 1000))
		ik.solvers = append(ik.solvers, nlopt)
	}
	ik.logger = logger
	return ik, nil
}

func runSolver(ctx context.Context, solver InverseKinematics, c chan []frame.Input, pos spatialmath.Pose, seed []frame.Input) error {
	return solver.Solve(ctx, c, pos, seed)
}

// Solve will initiate solving for the given position in all child solvers, seeding with the specified initial joint
// positions. If unable to solve, the returned error will be non-nil
func (ik *CombinedIK) Solve(ctx context.Context, c chan []frame.Input, newGoal spatialmath.Pose, seed []frame.Input) error {
	ik.logger.Debugf("starting joint positions: %v", seed)
	startPos, err := ik.model.Transform(seed)
	if err != nil {
		return err
	}
	// This will adjust the goal position to make movements more intuitive when using incrementation near poles
	ik.logger.Debugf("starting 6d position: %v", spatialmath.PoseToProtobuf(startPos))
	ik.logger.Debugf("goal 6d position: %v", spatialmath.PoseToProtobuf(newGoal))

	ctxWithCancel, cancel := context.WithCancel(ctx)

	errChan := make(chan error)
	var activeSolvers sync.WaitGroup
	activeSolvers.Add(len(ik.solvers))

	for _, solver := range ik.solvers {
		thisSolver := solver

		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()
			errChan <- runSolver(ctxWithCancel, thisSolver, c, newGoal, seed)
		})
	}

	returned := 0
	done := false

	var collectedErrs error

	// Wait until either 1) we have a success or 2) all solvers have returned false
	for !done {
		//~ fmt.Println(returned)
		select {
		case <-ctx.Done():
			done = true
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
		err = <-errChan
		returned++
		if err != nil {
			collectedErrs = multierr.Combine(collectedErrs, err)
		}
	}
	activeSolvers.Wait()
	close(errChan)
	return collectedErrs
}

// Frame returns the associated frame
func (ik *CombinedIK) Frame() frame.Frame {
	return ik.model
}

// Close closes all member IK solvers
func (ik *CombinedIK) Close() error {
	var err error
	for _, solver := range ik.solvers {
		err = multierr.Combine(err, solver.Close())
	}
	return err
}

// SetSolveWeights sets the solve weights for the solver.
func (ik *CombinedIK) SetSolveWeights(weights frame.SolverDistanceWeights) {
	for _, solver := range ik.solvers {
		solver.SetSolveWeights(weights)
	}
}

// SetGradient sets the function for distance between two poses
func (ik *CombinedIK) SetGradient(f func(spatialmath.Pose, spatialmath.Pose) float64) {
	for _, solver := range ik.solvers {
		solver.SetGradient(f)
	}
}
