package kinematics

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/go-nlopt/nlopt"
	"go.uber.org/multierr"

	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// NloptIK TODO
type NloptIK struct {
	id            int
	model         frame.Frame
	lowerBound    []float64
	upperBound    []float64
	iterations    int
	maxIterations int
	epsilon       float64
	goals         []goal
	opt           *nlopt.NLopt
	logger        golog.Logger
	jump          float64
	randSeed      *rand.Rand
	SolveWeights  frame.SolverDistanceWeights
}

// CreateNloptIKSolver TODO
func CreateNloptIKSolver(mdl frame.Frame, logger golog.Logger, id int) (*NloptIK, error) {
	ik := &NloptIK{id: id, logger: logger}
	ik.randSeed = rand.New(rand.NewSource(1))
	ik.model = mdl
	// How close we want to get to the goal
	ik.epsilon = 0.001
	// The absolute smallest value able to be represented by a float64
	floatEpsilon := math.Nextafter(1, 2) - 1
	ik.maxIterations = 5000
	ik.iterations = 0
	ik.lowerBound, ik.upperBound = limitsToArrays(mdl.DoF())
	// How much to adjust joints to determine slope
	ik.jump = 0.00000001

	ik.SolveWeights = frame.SolverDistanceWeights{frame.XYZWeights{1.0, 1.0, 1.0}, frame.XYZTHWeights{1.0, 1.0, 1.0, 1.0}}

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ik.model.DoF())))
	if err != nil {
		return nil, fmt.Errorf("nlopt creation error: %w", err)
	}
	ik.opt = opt

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {

		ik.iterations++

		// TODO(pl): Might need to check if any of x is +/- Inf
		eePos, err := ik.model.Transform(frame.FloatsToInputs(x))
		if err != nil && eePos == nil {
			ik.logger.Errorf("error calculating eePos in nlopt %q", err)
			err = ik.opt.ForceStop()
			ik.logger.Errorf("forcestop error %q", err)
		}
		dx := make([]float64, 6)

		// Update dx with the delta to the desired position
		for _, nextGoal := range ik.getGoals() {
			dxDelta := spatial.PoseDelta(eePos, nextGoal.GoalTransform)

			dxIdx := nextGoal.EffectorID * len(dxDelta)
			for i, delta := range dxDelta {
				dx[dxIdx+i] = delta
			}
		}

		dist := WeightedSquaredNorm(dx, ik.SolveWeights)

		if len(gradient) > 0 {
			for i := range gradient {
				// Deep copy of our current joint positions
				xBak := append([]float64{}, x...)
				xBak[i] += ik.jump
				eePos, err := ik.model.Transform(frame.FloatsToInputs(xBak))
				if err != nil && eePos == nil {
					ik.logger.Errorf("error calculating eePos in nlopt %q", err)
					err = ik.opt.ForceStop()
					ik.logger.Errorf("forcestop error %q", err)
				}
				dx2 := make([]float64, 6)
				for _, nextGoal := range ik.getGoals() {
					dxDelta := spatial.PoseDelta(eePos, nextGoal.GoalTransform)
					dxIdx := nextGoal.EffectorID * len(dxDelta)
					for i, delta := range dxDelta {
						dx2[dxIdx+i] = delta
					}
				}
				dist2 := WeightedSquaredNorm(dx2, ik.SolveWeights)

				gradient[i] = (dist2 - dist) / (2 * ik.jump)
			}
		}
		return dist
	}
	if len(ik.lowerBound) == 0 || len(ik.upperBound) == 0 {
		return nil, errors.New("cannot set upper or lower bounds for nlopt, slice is empty")
	}

	err = multierr.Combine(
		opt.SetFtolAbs(floatEpsilon),
		opt.SetFtolRel(floatEpsilon),
		opt.SetLowerBounds(ik.lowerBound),
		opt.SetMinObjective(nloptMinFunc),
		opt.SetStopVal(ik.epsilon*ik.epsilon),
		opt.SetUpperBounds(ik.upperBound),
		opt.SetXtolAbs1(floatEpsilon),
		opt.SetXtolRel(floatEpsilon),
		opt.SetMaxEval(8001),
	)

	if err != nil {
		return nil, err
	}

	return ik, nil
}

// addGoal adds a nlopt IK goal
func (ik *NloptIK) addGoal(newGoal *pb.Pose, effectorID int) {

	goalQuat := spatial.NewPoseFromProtobuf(newGoal)
	ik.goals = append(ik.goals, goal{goalQuat, effectorID})
}

// clearGoals clears all goals for the Ik object
func (ik *NloptIK) clearGoals() {
	ik.goals = []goal{}
}

// GetGoals returns the list of all current goal positions
func (ik *NloptIK) getGoals() []goal {
	return ik.goals
}

// SetSolveWeights sets the slve weights
func (ik *NloptIK) SetSolveWeights(weights frame.SolverDistanceWeights) {
	ik.SolveWeights = weights
}

// Solve runs the actual solver and returns a list of all
func (ik *NloptIK) Solve(ctx context.Context, newGoal *pb.Pose, seed []frame.Input) ([]frame.Input, error) {
	var err error

	// Allow ~160 degrees of swing at most
	tries := 1
	ik.iterations = 0
	startingPos := ik.GenerateRandomPositions()

	// Solver with ID 1 seeds off current angles
	if ik.id == 1 {
		if len(seed) > len(ik.model.DoF()) {
			return nil, errors.New("passed in too many joint positions")
		}
		startingPos = seed

		// Set initial restrictions on joints for more intuitive movement
		err = ik.updateBounds(startingPos, tries)
		if err != nil {
			return nil, err
		}
	} else {
		//~ // Solvers whose ID is not 1 should skip ahead directly to trying random seeds
		tries = 30
	}
	ik.addGoal(newGoal, 0)
	defer ik.clearGoals()

	select {
	case <-ctx.Done():
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return nil, err
	default:
	}

	var solutions [][]frame.Input

	for ik.iterations < ik.maxIterations {
		retrySeed := false
		select {
		case <-ctx.Done():
			return nil, err
		default:
		}
		ik.iterations++
		solutionRaw, result, nloptErr := ik.opt.Optimize(frame.InputsToFloats(startingPos))
		if nloptErr != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			err = multierr.Combine(err, nloptErr)
		}

		if result < ik.epsilon*ik.epsilon {
			solution := frame.FloatsToInputs(solutionRaw)
			// Return immediately if we have a "natural" solution, i.e. one where the halfway point is on the way
			// to the end point
			swing, newErr := calcSwingAmount(seed, solution, ik.model)
			if newErr != nil {
				// out-of-bounds angles. Shouldn't happen, but if it does, record the error and move on without
				// keeping the invalid solution
				err = multierr.Combine(err, newErr)
			} else if swing < goodSwingAmt {
				return solution, err
			} else {
				solutions = append(solutions, solution)
			}
		}
		tries++
		if tries < 30 {
			err = ik.updateBounds(seed, tries)
			if err != nil {
				return nil, err
			}
		} else if !retrySeed {
			err = multierr.Combine(
				ik.opt.SetLowerBounds(ik.lowerBound),
				ik.opt.SetUpperBounds(ik.upperBound),
			)
			if err != nil {
				return nil, err
			}
			startingPos = ik.GenerateRandomPositions()
		}
	}
	if len(solutions) > 0 {
		solution, _, err := bestSolution(seed, solutions, ik.model)
		return solution, err
	}
	return nil, multierr.Combine(errors.New("kinematics could not solve for position"), err)
}

// SetSeed sets the random seed of this solver
func (ik *NloptIK) SetSeed(seed int64) {
	ik.randSeed = rand.New(rand.NewSource(seed))
}

// GenerateRandomPositions generates a random set of positions within the limits of this solver.
func (ik *NloptIK) GenerateRandomPositions() []frame.Input {
	pos := make([]frame.Input, len(ik.model.DoF()))
	for i, l := range ik.lowerBound {
		u := ik.upperBound[i]

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		jRange := math.Abs(u - l)
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		pos[i] = frame.Input{ik.randSeed.Float64()*jRange + l}
	}
	return pos
}

// Model returns the associated model
func (ik *NloptIK) Model() frame.Frame {
	return ik.model
}

// Close destroys the C nlopt object to prevent memory leaks.
func (ik *NloptIK) Close() error {
	err := ik.opt.ForceStop()
	ik.opt.Destroy()
	return err
}

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *NloptIK) updateBounds(seed []frame.Input, tries int) error {
	rangeStep := 0.1
	newLower := make([]float64, len(ik.lowerBound))
	newUpper := make([]float64, len(ik.upperBound))

	for i, pos := range seed {
		newLower[i] = math.Max(ik.lowerBound[i], pos.Value-(rangeStep*float64(tries*(i+1))))
		newUpper[i] = math.Min(ik.upperBound[i], pos.Value+(rangeStep*float64(tries*(i+1))))

		// Allow full freedom of movement for the two most distal joints
		if i > len(seed)-2 {
			newLower[i] = ik.lowerBound[i]
			newUpper[i] = ik.upperBound[i]
		}
	}
	return multierr.Combine(
		ik.opt.SetLowerBounds(newLower),
		ik.opt.SetUpperBounds(newUpper),
	)
}
