//go:build !windows && !no_cgo

package ik

import (
	"context"
	"math"
	"math/rand"
	"sync"

	"github.com/go-nlopt/nlopt"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

var (
	errNoSolve     = errors.New("kinematics could not solve for position")
	errBadBounds   = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")
	errTooManyVals = errors.New("passed in too many joint positions")
)

const (
	nloptStepsPerIter = 4001
	defaultMaxIter    = 5000
	defaultJump       = 1e-8
)

// NloptIK TODO.
type NloptIK struct {
	id            int
	minFunc       func([]float64) float64
	lowerBound    []float64
	upperBound    []float64
	maxIterations int
	epsilon       float64
	logger        logging.Logger

	// Nlopt will try to minimize a configuration for whatever is passed in. If exact is false, then the solver will emit partial
	// solutions where it was not able to meet the goal criteria but still was able to improve upon the seed.
	exact bool

	// useRelTol specifies whether the SetXtolRel and SetFtolRel values will be set for nlopt.
	// If true, this will terminate solving when nlopt alg iterations change the distance to goal by less than some proportion of calculated
	// distance. This can cause premature terminations when the distances are large.
	useRelTol bool
}

type optimizeReturn struct {
	solution []float64
	score    float64
	err      error
}

// CreateNloptIKSolver creates an nloptIK object that can perform gradient descent on metrics for Frames. The parameters are the Frame on
// which Transform() will be called, a logger, and the number of iterations to run. If the iteration count is less than 1, it will be set
// to the default of 5000.
func CreateNloptIKSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
) (*NloptIK, error) {
	ik := &NloptIK{logger: logger}
	ik.id = 0

	// Stop optimizing when iterations change by less than this much
	// Also, how close we want to get to the goal region. The metric should reflect any buffer.
	ik.epsilon = defaultEpsilon * defaultEpsilon
	if iter < 1 {
		// default value
		iter = defaultMaxIter
	}
	ik.maxIterations = iter
	ik.lowerBound, ik.upperBound = limitsToArrays(limits)
	ik.exact = exact
	ik.useRelTol = useRelTol

	return ik, nil
}

// Solve runs the actual solver and sends any solutions found to the given channel.
func (ik *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *Solution,
	seed []float64,
	minFunc func([]float64) float64,
	rseed int,
) error {
	//nolint: gosec
	randSeed := rand.New(rand.NewSource(int64(rseed)))
	var err error

	// Determine optimal jump values; start with default, and if gradient is zero, increase to 1 to try to avoid underflow.
	jump, err := ik.calcJump(defaultJump, seed, minFunc)
	if err != nil {
		return err
	}

	iterations := 0
	solutionsFound := 0

	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ik.lowerBound)))
	defer opt.Destroy()
	if err != nil {
		return errors.Wrap(err, "nlopt creation error")
	}

	if len(ik.lowerBound) == 0 || len(ik.upperBound) == 0 {
		return errBadBounds
	}
	var activeSolvers sync.WaitGroup

	jumpVal := 0.

	// x is our set of inputs
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {
		iterations++
		dist := minFunc(x)
		if len(gradient) > 0 {
			// Yes, the for loop below is logically equivalent to not having this if statement. But CPU branch prediction means having the
			// if statement is faster.
			for i := range gradient {
				jumpVal = jump[i]
				flip := false
				x[i] += jumpVal
				ub := ik.upperBound[i]
				if x[i] >= ub {
					flip = true
					x[i] -= 2 * jumpVal
				}

				dist2 := minFunc(x)
				gradient[i] = (dist2 - dist) / jumpVal
				if flip {
					x[i] += jumpVal
					gradient[i] *= -1
				} else {
					x[i] -= jumpVal
				}
			}
		}
		return dist
	}

	err = multierr.Combine(
		opt.SetFtolAbs(ik.epsilon),
		opt.SetLowerBounds(ik.lowerBound),
		opt.SetStopVal(ik.epsilon),
		opt.SetUpperBounds(ik.upperBound),
		opt.SetXtolAbs1(ik.epsilon),
		opt.SetMinObjective(nloptMinFunc),
		opt.SetMaxEval(nloptStepsPerIter),
	)
	if ik.useRelTol {
		err = multierr.Combine(
			err,
			opt.SetFtolRel(ik.epsilon),
			opt.SetXtolRel(ik.epsilon),
		)
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
			solutionRaw, result, nloptErr := opt.Optimize(seed)
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
		if err != nil {
			return err
		}
		seed = generateRandomPositions(randSeed, ik.lowerBound, ik.upperBound)
	}
	if solutionsFound > 0 {
		return nil
	}
	return multierr.Combine(err, errNoSolve)
}

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *NloptIK) updateBounds(seed []float64, tries int, opt *nlopt.NLopt) error {
	rangeStep := 0.1
	newLower := make([]float64, len(ik.lowerBound))
	newUpper := make([]float64, len(ik.upperBound))

	for i, pos := range seed {
		newLower[i] = math.Max(ik.lowerBound[i], pos-(rangeStep*float64(tries*(i+1))))
		newUpper[i] = math.Min(ik.upperBound[i], pos+(rangeStep*float64(tries*(i+1))))

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

func (ik *NloptIK) calcJump(testJump float64, seed []float64, minFunc func([]float64) float64) ([]float64, error) {
	jump := make([]float64, 0, len(seed))
	seedDist := minFunc(seed)
	for i, testVal := range seed {
		seedTest := append(make([]float64, 0, len(seed)), seed...)
		for jumpVal := testJump; jumpVal < 0.1; jumpVal *= 10 {
			seedTest[i] = testVal + jumpVal
			if seedTest[i] > ik.upperBound[i] {
				seedTest[i] = testVal - jumpVal
				if seedTest[i] < ik.lowerBound[i] {
					jump = append(jump, testJump)
					break
				}
			}

			checkDist := minFunc(seed)

			// Use the smallest value that yields a change in distance
			if checkDist != seedDist {
				jump = append(jump, jumpVal)
				break
			}
		}
		if len(jump) != i+1 {
			jump = append(jump, testJump)
		}
	}
	return jump, nil
}
