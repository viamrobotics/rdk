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

	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

var (
	errNoSolve     = errors.New("kinematics could not solve for position")
	errBadBounds   = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")
	errTooManyVals = errors.New("passed in too many joint positions")
)

const constrainedTries = 30

// NloptIK TODO
type NloptIK struct {
	id            int
	model         frame.Frame
	lowerBound    []float64
	upperBound    []float64
	iterations    int
	maxIterations int
	epsilon       float64
	goal          goal
	opt           *nlopt.NLopt
	logger        golog.Logger
	jump          float64
	randSeed      *rand.Rand
	metric        Metric
}

// CreateNloptIKSolver TODO
func CreateNloptIKSolver(mdl frame.Frame, logger golog.Logger) (*NloptIK, error) {
	ik := &NloptIK{logger: logger}
	ik.randSeed = rand.New(rand.NewSource(1))
	ik.model = mdl
	ik.id = 0
	// How close we want to get to the goal
	ik.epsilon = 0.001
	// The absolute smallest value able to be represented by a float64
	floatEpsilon := math.Pow(ik.epsilon, 4)
	ik.maxIterations = 5000
	ik.iterations = 0
	ik.lowerBound, ik.upperBound = limitsToArrays(mdl.DoF())
	// How much to adjust joints to determine slope
	ik.jump = 0.00000001

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ik.model.DoF())))
	if err != nil {
		return nil, fmt.Errorf("nlopt creation error: %w", err)
	}
	ik.opt = opt
	ik.metric = NewSquaredNormMetric()

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

		dist := ik.metric(eePos, ik.goal.GoalTransform)

		if len(gradient) > 0 {
			for i := range gradient {
				x[i] += ik.jump
				eePos, err := ik.model.Transform(frame.FloatsToInputs(x))
				x[i] -= ik.jump
				if err != nil && eePos == nil {
					ik.logger.Errorf("error calculating eePos in nlopt %q", err)
					err = ik.opt.ForceStop()
					ik.logger.Errorf("forcestop error %q", err)
				}
				dist2 := ik.metric(eePos, ik.goal.GoalTransform)

				gradient[i] = (dist2 - dist) / (2 * ik.jump)
			}
		}
		return dist
	}
	if len(ik.lowerBound) == 0 || len(ik.upperBound) == 0 {
		return nil, errBadBounds
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
		opt.SetMaxEval(4001),
	)

	if err != nil {
		return nil, err
	}

	return ik, nil
}

// addGoal adds a nlopt IK goal
func (ik *NloptIK) addGoal(newGoal spatial.Pose, effectorID int) {

	ik.goal = goal{newGoal, effectorID}
}

// clearGoals clears all goals for the Ik object
func (ik *NloptIK) clearGoal() {
	ik.goal = goal{}
}

// SetMetric sets the function for distance between two poses
func (ik *NloptIK) SetMetric(m Metric) {
	ik.metric = m
}

// SetMaxIter sets the number of times the solver will iterate
func (ik *NloptIK) SetMaxIter(i int) {
	ik.maxIterations = i
}

// Solve runs the actual solver and returns a list of all
func (ik *NloptIK) Solve(ctx context.Context, c chan<- []frame.Input, newGoal spatial.Pose, seed []frame.Input) error {
	var err error

	// Allow ~160 degrees of swing at most
	tries := 1
	ik.iterations = 0
	solutionsFound := 0
	startingPos := seed
	if ik.id > 0 {

		// Solver with ID 1 seeds off current angles
		if ik.id == 1 {
			if len(seed) > len(ik.model.DoF()) {
				return errTooManyVals
			}
			startingPos = seed

			// Set initial restrictions on joints for more intuitive movement
			err = ik.updateBounds(startingPos, tries)
			if err != nil {
				return err
			}
		} else {
			// Solvers whose ID is not 1 should skip ahead directly to trying random seeds
			startingPos = ik.GenerateRandomPositions()
			tries = constrainedTries
		}
	}
	ik.addGoal(newGoal, 0)
	defer ik.clearGoal()

	select {
	case <-ctx.Done():
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return err
	default:
	}

	for ik.iterations < ik.maxIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
			select {
			case <-ctx.Done():
				return err
			case c <- frame.FloatsToInputs(solutionRaw):
			}
			solutionsFound++
		}
		tries++
		if ik.id > 0 && tries < constrainedTries {
			err = ik.updateBounds(seed, tries)
			if err != nil {
				return err
			}
		} else {
			err = multierr.Combine(
				ik.opt.SetLowerBounds(ik.lowerBound),
				ik.opt.SetUpperBounds(ik.upperBound),
			)
			if err != nil {
				return err
			}
			startingPos = ik.GenerateRandomPositions()
		}
	}
	if solutionsFound > 0 {
		return nil
	}
	return multierr.Combine(err, errNoSolve)
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

// Frame returns the associated frame
func (ik *NloptIK) Frame() frame.Frame {
	return ik.model
}

// Close destroys the C nlopt object to prevent memory leaks.
func (ik *NloptIK) Close() error {
	err := ik.opt.ForceStop()
	ik.opt.Destroy()
	return err
}

// UpdateBounds updates the lower/upper bounds
func (ik *NloptIK) UpdateBounds(lower, upper []float64) error {
	return multierr.Combine(
		ik.opt.SetLowerBounds(lower),
		ik.opt.SetUpperBounds(upper),
	)
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
