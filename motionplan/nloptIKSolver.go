package motionplan

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"strings"

	"github.com/edaniels/golog"
	"github.com/go-nlopt/nlopt"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

var (
	errNoSolve     = errors.New("kinematics could not solve for position")
	errBadBounds   = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")
	errTooManyVals = errors.New("passed in too many joint positions")
)

const (
	defaultConstrainedTries = 30
	defaultStepsPerIter     = 4001
	defaultMaxIterations    = 5000

	// Default distance below which two distances are considered equal.
	defaultEpsilon = 1e-3

	defaultSolveEpsilon = 1e-12
	defaultJump         = 1e-8
)

type nloptOptions struct {
	ConstrainedTries  int `json:"constrained_tries"`
	StepsPerIteration int `json:"steps_per_iteration"`
	MaxIterations     int `json:"iterations"`

	// How close we want to get to the goal
	Epsilon float64 `json:"epsilon"`

	// Stop optimizing when iterations change by less than this much
	SolveEpsilon float64 `json:"solve_epsilon"`

	// How much to adjust joints to determine slope
	Jump float64 `json:"jump"`
}

func newNLOptOptions(opt *ikOptions) (*nloptOptions, error) {
	algOpts := &nloptOptions{
		ConstrainedTries:  defaultConstrainedTries,
		StepsPerIteration: defaultStepsPerIter,
		MaxIterations:     defaultMaxIterations,
		Epsilon:           defaultEpsilon,
		SolveEpsilon:      defaultSolveEpsilon,
		Jump:              defaultJump,
	}
	// convert map to json
	jsonString, err := json.Marshal(opt.extra)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, algOpts)
	if err != nil {
		return nil, err
	}
	return algOpts, nil
}

type nloptIKSolver struct {
	*ikSolver
	algOpts    *nloptOptions
	id         int
	lowerBound []float64
	upperBound []float64
}

// newNLOptIKSolver creates an nloptIK object that can perform gradient descent on metrics for Frames. The parameters are the Frame on
// which Transform() will be called, a logger, and the number of iterations to run. If the iteration count is less than 1, it will be set
// to the default of 5000.
func newNLOptIKSolver(model referenceframe.Frame, logger golog.Logger, opts *ikOptions) (*nloptIKSolver, error) {
	if opts == nil {
		opts = newBasicIKOptions()
	}
	algOpts, err := newNLOptOptions(opts)
	if err != nil {
		return nil, err
	}
	ik := &nloptIKSolver{
		ikSolver: &ikSolver{
			logger: logger,
			model:  model,
			opts:   opts,
		},
		algOpts: algOpts,
		id:      0,
	}
	for _, limit := range model.DoF() {
		ik.lowerBound = append(ik.lowerBound, limit.Min)
		ik.upperBound = append(ik.upperBound, limit.Max)
	}
	return ik, nil
}

// Solve runs the actual solver and sends any solutions found to the given channel.
func (ik *nloptIKSolver) solve(ctx context.Context,
	c chan<- []referenceframe.Input,
	newGoal spatial.Pose,
	seed []referenceframe.Input,
	m Metric,
	rseed int,
) error {
	//nolint: gosec
	randSeed := rand.New(rand.NewSource(int64(rseed)))
	var err error

	tries := 1
	iterations := 0
	solutionsFound := 0
	startingPos := seed

	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ik.model.DoF())))
	defer opt.Destroy()
	if err != nil {
		return errors.Wrap(err, "nlopt creation error")
	}

	if len(ik.lowerBound) == 0 || len(ik.upperBound) == 0 {
		return errBadBounds
	}

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {
		iterations++

		// Requesting an out-of-bounds transform will result in a non-nil error but will optionally return a correct if invalid pose.
		// Thus we check if eePos is nil, and if not, continue as normal and ignore errors.
		// As confirmation, the "input out of bounds" string is checked for in the error text.
		eePos, err := ik.model.Transform(referenceframe.FloatsToInputs(x))
		if eePos == nil || (err != nil && !strings.Contains(err.Error(), referenceframe.OOBErrString)) {
			ik.logger.Errorw("error calculating eePos in nlopt", "error", err)
			err = opt.ForceStop()
			ik.logger.Errorw("forcestop error", "error", err)
			return 0
		}

		dist := m(eePos, newGoal)

		if len(gradient) > 0 {
			for i := range gradient {
				x[i] += ik.algOpts.Jump
				eePos, err := ik.model.Transform(referenceframe.FloatsToInputs(x))
				x[i] -= ik.algOpts.Jump
				if eePos == nil || (err != nil && !strings.Contains(err.Error(), referenceframe.OOBErrString)) {
					ik.logger.Errorw("error calculating eePos in nlopt", "error", err)
					err = opt.ForceStop()
					ik.logger.Errorw("forcestop error", "error", err)
					return 0
				}
				dist2 := m(eePos, newGoal)

				gradient[i] = (dist2 - dist) / ik.algOpts.Jump
			}
		}
		return dist
	}

	err = multierr.Combine(
		opt.SetFtolAbs(ik.algOpts.SolveEpsilon),
		opt.SetFtolRel(ik.algOpts.SolveEpsilon),
		opt.SetLowerBounds(ik.lowerBound),
		opt.SetStopVal(ik.algOpts.Epsilon*ik.algOpts.Epsilon),
		opt.SetUpperBounds(ik.upperBound),
		opt.SetXtolAbs1(ik.algOpts.SolveEpsilon),
		opt.SetXtolRel(ik.algOpts.SolveEpsilon),
		opt.SetMinObjective(nloptMinFunc),
		opt.SetMaxEval(ik.algOpts.StepsPerIteration),
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
			startingPos = ik.GenerateRandomPositions(randSeed)
			tries = ik.algOpts.ConstrainedTries
		}
	}

	select {
	case <-ctx.Done():
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return err
	default:
	}

	for iterations < ik.algOpts.MaxIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		iterations++
		solutionRaw, result, nloptErr := opt.Optimize(referenceframe.InputsToFloats(startingPos))
		if nloptErr != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			err = multierr.Combine(err, nloptErr)
		}

		if result < ik.algOpts.Epsilon*ik.algOpts.Epsilon {
			select {
			case <-ctx.Done():
				return err
			case c <- referenceframe.FloatsToInputs(solutionRaw):
			}
			solutionsFound++
		}
		tries++
		if ik.id > 0 && tries < ik.algOpts.ConstrainedTries {
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
			startingPos = ik.GenerateRandomPositions(randSeed)
		}
	}
	if solutionsFound > 0 {
		return nil
	}
	return multierr.Combine(err, errNoSolve)
}

// GenerateRandomPositions generates a random set of positions within the limits of this solver.
func (ik *nloptIKSolver) GenerateRandomPositions(randSeed *rand.Rand) []referenceframe.Input {
	pos := make([]referenceframe.Input, len(ik.model.DoF()))
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
		pos[i] = referenceframe.Input{randSeed.Float64()*jRange + l}
	}
	return pos
}

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *nloptIKSolver) updateBounds(seed []referenceframe.Input, tries int, opt *nlopt.NLopt) error {
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
