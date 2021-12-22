package motionplan

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
const nloptStepsPerIter = 4001

// NloptIK TODO
type NloptIK struct {
	id            int
	model         frame.Frame
	lowerBound    []float64
	upperBound    []float64
	maxIterations int
	epsilon       float64
	solveEpsilon  float64
	logger        golog.Logger
	jump          float64
	randSeed      *rand.Rand
}

// CreateNloptIKSolver TODO
func CreateNloptIKSolver(mdl frame.Frame, logger golog.Logger) (*NloptIK, error) {
	ik := &NloptIK{logger: logger}
	ik.randSeed = rand.New(rand.NewSource(1))
	ik.model = mdl
	ik.id = 0
	// How close we want to get to the goal
	ik.epsilon = 0.001
	// Stop optimizing when iterations change by less than this much
	ik.solveEpsilon = math.Pow(ik.epsilon, 4)
	ik.maxIterations = 5000
	ik.lowerBound, ik.upperBound = limitsToArrays(mdl.DoF())
	// How much to adjust joints to determine slope
	ik.jump = 0.00000001

	return ik, nil
}

// SetMaxIter sets the number of times the solver will iterate
func (ik *NloptIK) SetMaxIter(i int) {
	ik.maxIterations = i
}

// Solve runs the actual solver and returns a list of all
func (ik *NloptIK) Solve(ctx context.Context, c chan<- []frame.Input, newGoal spatial.Pose, seed []frame.Input, m Metric) error {
	var err error

	// Allow ~160 degrees of swing at most
	tries := 1
	iterations := 0
	solutionsFound := 0
	startingPos := seed

	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ik.model.DoF())))
	defer opt.Destroy()
	if err != nil {
		return fmt.Errorf("nlopt creation error: %w", err)
	}

	if len(ik.lowerBound) == 0 || len(ik.upperBound) == 0 {
		return errBadBounds
	}

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {

		iterations++

		// TODO(pl): Might need to check if any of x is +/- Inf
		eePos, err := ik.model.Transform(frame.FloatsToInputs(x))
		if eePos == nil {
			ik.logger.Errorf("error calculating eePos in nlopt %q", err)
			err = opt.ForceStop()
			ik.logger.Errorf("forcestop error %q", err)
		}

		dist := m(eePos, newGoal)

		if len(gradient) > 0 {
			xBak := append([]float64{}, x...)
			for i := range gradient {
				xBak[i] += ik.jump
				eePos, err := ik.model.Transform(frame.FloatsToInputs(xBak))
				xBak[i] -= ik.jump
				if eePos == nil {
					ik.logger.Errorf("error calculating eePos in nlopt %q", err)
					err = opt.ForceStop()
					ik.logger.Errorf("forcestop error %q", err)
				}
				dist2 := m(eePos, newGoal)

				gradient[i] = (dist2 - dist) / ik.jump
			}
		}
		return dist
	}

	err = multierr.Combine(
		opt.SetFtolAbs(ik.solveEpsilon),
		opt.SetFtolRel(ik.solveEpsilon),
		opt.SetLowerBounds(ik.lowerBound),
		opt.SetStopVal(ik.epsilon*ik.epsilon),
		opt.SetUpperBounds(ik.upperBound),
		opt.SetXtolAbs1(ik.solveEpsilon),
		opt.SetXtolRel(ik.solveEpsilon),
		opt.SetMinObjective(nloptMinFunc),
		opt.SetMaxEval(nloptStepsPerIter),
	)

	if ik.id > 0 {

		// Solver with ID 1 seeds off current angles
		if ik.id == 1 {
			if len(seed) > len(ik.model.DoF()) {
				return errTooManyVals
			}
			startingPos = seed

			// Set initial restrictions on joints for more intuitive movement
			err = ik.updateBounds(startingPos, tries, opt)
			if err != nil {
				return err
			}
		} else {
			// Solvers whose ID is not 1 should skip ahead directly to trying random seeds
			startingPos = ik.GenerateRandomPositions()
			tries = constrainedTries
		}
	}

	select {
	case <-ctx.Done():
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return err
	default:
	}

	for iterations < ik.maxIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		iterations++
		solutionRaw, result, nloptErr := opt.Optimize(frame.InputsToFloats(startingPos))
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
			err = ik.updateBounds(seed, tries, opt)
			if err != nil {
				return err
			}
		} else {
			err = multierr.Combine(
				opt.SetLowerBounds(ik.lowerBound),
				opt.SetUpperBounds(ik.upperBound),
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

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *NloptIK) updateBounds(seed []frame.Input, tries int, opt *nlopt.NLopt) error {
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
		opt.SetLowerBounds(newLower),
		opt.SetUpperBounds(newUpper),
	)
}
