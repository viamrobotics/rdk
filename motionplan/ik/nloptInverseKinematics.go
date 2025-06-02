//go:build !windows && !no_cgo

package ik

import (
	"context"
	"fmt"
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
	errNoSolve   = errors.New("kinematics could not solve for position")
	errBadBounds = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")
)

const (
	nloptStepsPerIter = 4001
	defaultMaxIter    = 5000
	defaultJump       = 1e-8
)

type nloptIK struct {
	id            int
	limits        []referenceframe.Limit
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

// CreateNloptSolver creates an nloptIK object that can perform gradient descent on functions. The parameters are the limits
// of the solver, a logger, and the number of iterations to run. If the iteration count is less than 1, it will be set
// to the default of 5000.
func CreateNloptSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
) (Solver, error) {
	ik := &nloptIK{logger: logger, limits: limits}
	ik.id = 0

	// Stop optimizing when iterations change by less than this much
	// Also, how close we want to get to the goal region. The metric should reflect any buffer.
	ik.epsilon = defaultEpsilon * defaultEpsilon
	if iter < 1 {
		// default value
		iter = defaultMaxIter
	}
	ik.maxIterations = iter
	ik.exact = exact
	ik.useRelTol = useRelTol

	return ik, nil
}

// DoF returns the DoF of the solver.
func (ik *nloptIK) DoF() []referenceframe.Limit {
	return ik.limits
}

// Solve runs the actual solver and sends any solutions found to the given channel.
func (ik *nloptIK) Solve(ctx context.Context,
	solutionChan chan<- *Solution,
	seed []float64,
	minFunc func([]float64) float64,
	rseed int,
) error {
	if len(seed) != len(ik.limits) {
		return fmt.Errorf("nlopt initialized with %d dof but seed was length %d", len(ik.limits), len(seed))
	}
	//nolint: gosec
	randSeed := rand.New(rand.NewSource(int64(rseed)))
	var err error

	// Determine optimal jump values; start with default, and if gradient is zero, increase to 1 to try to avoid underflow.
	jump := ik.calcJump(defaultJump, seed, minFunc)

	iterations := 0
	solutionsFound := 0

	lowerBound, upperBound := limitsToArrays(ik.limits)
	if len(lowerBound) == 0 || len(upperBound) == 0 {
		return errBadBounds
	}

	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(lowerBound)))
	defer opt.Destroy()
	if err != nil {
		return errors.Wrap(err, "nlopt creation error")
	}

	var activeSolvers sync.WaitGroup

	jumpVal := 0.

	// checkVals is our set of inputs that we evaluate for distance
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(checkVals, gradient []float64) float64 {
		iterations++
		dist := minFunc(checkVals)
		if len(gradient) > 0 {
			// Yes, the for loop below is logically equivalent to not having this if statement. But CPU branch prediction means having the
			// if statement is faster.
			for i := range gradient {
				jumpVal = jump[i]
				flip := false
				checkVals[i] += jumpVal
				ub := upperBound[i]
				if checkVals[i] >= ub {
					flip = true
					checkVals[i] -= 2 * jumpVal
				}

				dist2 := minFunc(checkVals)
				gradient[i] = (dist2 - dist) / jumpVal
				if flip {
					checkVals[i] += jumpVal
					gradient[i] *= -1
				} else {
					checkVals[i] -= jumpVal
				}
			}
		}
		return dist
	}

	err = multierr.Combine(
		opt.SetFtolAbs(ik.epsilon),
		opt.SetLowerBounds(lowerBound),
		opt.SetStopVal(ik.epsilon),
		opt.SetUpperBounds(upperBound),
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
				Configuration: solutionRaw,
				Score:         result,
				Exact:         result < ik.epsilon,
			}
			solutionsFound++
		}
		if err != nil {
			return err
		}
		seed = generateRandomPositions(randSeed, lowerBound, upperBound)
	}
	if solutionsFound > 0 {
		return nil
	}
	return multierr.Combine(err, errNoSolve)
}

func (ik *nloptIK) calcJump(testJump float64, seed []float64, minFunc func([]float64) float64) []float64 {
	jump := make([]float64, 0, len(seed))
	lowerBound, upperBound := limitsToArrays(ik.limits)

	seedDist := minFunc(seed)
	for i, testVal := range seed {
		seedTest := append(make([]float64, 0, len(seed)), seed...)
		for jumpVal := testJump; jumpVal < 0.1; jumpVal *= 10 {
			seedTest[i] = testVal + jumpVal
			if seedTest[i] > upperBound[i] {
				seedTest[i] = testVal - jumpVal
				if seedTest[i] < lowerBound[i] {
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
	return jump
}
