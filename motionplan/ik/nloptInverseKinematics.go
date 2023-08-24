//go:build !windows

package ik

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-nlopt/nlopt"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
)

var (
	errNoSolve     = errors.New("kinematics could not solve for position")
	errBadBounds   = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")
	errTooManyVals = errors.New("passed in too many joint positions")
)

const (
	constrainedTries  = 30
	nloptStepsPerIter = 4001
)

// NloptIK TODO.
type NloptIK struct {
	id            int
	model         referenceframe.Frame
	lowerBound    []float64
	upperBound    []float64
	maxIterations int
	epsilon       float64
	logger        golog.Logger
	jump          float64

	// Nlopt will try to minimize a configuration for whatever is passed in. If exact is false, then the solver will emit partial
	// solutions where it was not able to meet the goal criteria but still was able to improve upon the seed.
	exact bool
}

type optimizeReturn struct {
	solution []float64
	score    float64
	err      error
}

// CreateNloptIKSolver creates an nloptIK object that can perform gradient descent on metrics for Frames. The parameters are the Frame on
// which Transform() will be called, a logger, and the number of iterations to run. If the iteration count is less than 1, it will be set
// to the default of 5000.
func CreateNloptIKSolver(mdl referenceframe.Frame, logger golog.Logger, iter int, exact bool) (*NloptIK, error) {
	ik := &NloptIK{logger: logger}

	ik.model = mdl
	ik.id = 0

	// Stop optimizing when iterations change by less than this much
	// Also, how close we want to get to the goal region. The metric should reflect any buffer.
	ik.epsilon = defaultEpsilon * defaultEpsilon
	if iter < 1 {
		// default value
		iter = 5000
	}
	ik.maxIterations = iter
	ik.lowerBound, ik.upperBound = limitsToArrays(mdl.DoF())
	// How much to adjust joints to determine slope
	ik.jump = 0.00000001
	ik.exact = exact

	return ik, nil
}

// Solve runs the actual solver and sends any solutions found to the given channel.
func (ik *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *Solution,
	seed []referenceframe.Input,
	solveMetric StateMetric,
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
	mInput := &State{Frame: ik.model}
	var activeSolvers sync.WaitGroup

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {
		iterations++

		// Requesting an out-of-bounds transform will result in a non-nil error but will optionally return a correct if invalid pose.
		// Thus we check if eePos is nil, and if not, continue as normal and ignore errors.
		// As confirmation, the "input out of bounds" string is checked for in the error text.
		inputs := referenceframe.FloatsToInputs(x)
		eePos, err := ik.model.Transform(inputs)
		if eePos == nil || (err != nil && !strings.Contains(err.Error(), referenceframe.OOBErrString)) {
			ik.logger.Errorw("error calculating eePos in nlopt", "error", err)
			err = opt.ForceStop()
			ik.logger.Errorw("forcestop error", "error", err)
			return 0
		}
		mInput.Configuration = inputs
		mInput.Position = eePos
		dist := solveMetric(mInput)

		if len(gradient) > 0 {
			for i := range gradient {
				x[i] += ik.jump
				inputs = referenceframe.FloatsToInputs(x)
				eePos, err := ik.model.Transform(inputs)
				x[i] -= ik.jump
				if eePos == nil || (err != nil && !strings.Contains(err.Error(), referenceframe.OOBErrString)) {
					ik.logger.Errorw("error calculating eePos in nlopt", "error", err)
					err = opt.ForceStop()
					ik.logger.Errorw("forcestop error", "error", err)
					return 0
				}
				mInput.Configuration = inputs
				mInput.Position = eePos
				dist2 := solveMetric(mInput)

				gradient[i] = (dist2 - dist) / ik.jump
			}
		}
		return dist
	}

	err = multierr.Combine(
		opt.SetFtolAbs(ik.epsilon),
		opt.SetFtolRel(ik.epsilon),
		opt.SetLowerBounds(ik.lowerBound),
		opt.SetStopVal(ik.epsilon),
		opt.SetUpperBounds(ik.upperBound),
		opt.SetXtolAbs1(ik.epsilon),
		opt.SetXtolRel(ik.epsilon),
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
			startingPos = ik.GenerateRandomPositions(randSeed)
			tries = constrainedTries
		}
	}

	solveChan := make(chan *optimizeReturn, 1)
	defer close(solveChan)
	for iterations < ik.maxIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var solutionRaw []float64
		var result float64
		var nloptErr error

		iterations++
		activeSolvers.Add(1)
		utils.PanicCapturingGo(func() {
			defer activeSolvers.Done()
			solutionRaw, result, nloptErr := opt.Optimize(referenceframe.InputsToFloats(startingPos))
			solveChan <- &optimizeReturn{solutionRaw, result, nloptErr}
		})
		select {
		case <-ctx.Done():
			err = multierr.Combine(err, opt.ForceStop())
			activeSolvers.Wait()
			return multierr.Combine(err, ctx.Err())
		case solution := <-solveChan:
			solutionRaw = solution.solution
			result = solution.score
			nloptErr = solution.err
		}
		if nloptErr != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			err = multierr.Combine(err, nloptErr)
		}

		if result < ik.epsilon || (solutionRaw != nil && !ik.exact) {
			select {
			case <-ctx.Done():
				return err
			default:
			}
			solutionChan <- &Solution{
				Configuration: referenceframe.FloatsToInputs(solutionRaw),
				Score:         result,
				Exact:         result < ik.epsilon,
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
			startingPos = ik.GenerateRandomPositions(randSeed)
		}
	}
	if solutionsFound > 0 {
		return nil
	}
	return multierr.Combine(err, errNoSolve)
}

// GenerateRandomPositions generates a random set of positions within the limits of this solver.
func (ik *NloptIK) GenerateRandomPositions(randSeed *rand.Rand) []referenceframe.Input {
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

// Frame returns the associated referenceframe.
func (ik *NloptIK) Frame() referenceframe.Frame {
	return ik.model
}

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *NloptIK) updateBounds(seed []referenceframe.Input, tries int, opt *nlopt.NLopt) error {
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

func limitsToArrays(limits []referenceframe.Limit) ([]float64, []float64) {
	var min, max []float64
	for _, limit := range limits {
		min = append(min, limit.Min)
		max = append(max, limit.Max)
	}
	return min, max
}
