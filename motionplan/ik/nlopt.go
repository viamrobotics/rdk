//go:build !windows && !no_cgo

package ik

import (
	"context"
	"fmt"
	"math/rand"
	"slices"

	"github.com/go-nlopt/nlopt"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

var errBadBounds = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")

const (
	nloptStepsPerIter = 4001
	defaultMaxIter    = 5000
	defaultJump       = 1e-8
)

// NloptIK can solve IK problems with nlopt.
type NloptIK struct {
	limits        []referenceframe.Limit
	maxIterations int
	logger        logging.Logger

	// Nlopt will try to minimize a configuration for whatever is passed in. If exact is false, then the solver will emit partial
	// solutions where it was not able to meet the goal criteria but still was able to improve upon the seed.
	exact bool

	// useRelTol specifies whether the SetXtolRel and SetFtolRel values will be set for nlopt.
	// If true, this will terminate solving when nlopt alg iterations change the distance to goal by less than some proportion of calculated
	// distance. This can cause premature terminations when the distances are large.
	useRelTol bool
}

// CreateNloptSolver creates an nloptIK object that can perform gradient descent on functions. The parameters are the limits
// of the solver, a logger, and the number of iterations to run. If the iteration count is less than 1, it will be set
// to the default of 5000.
func CreateNloptSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
) (*NloptIK, error) {
	ik := &NloptIK{logger: logger, limits: limits}

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
func (ik *NloptIK) DoF() []referenceframe.Limit {
	return ik.limits
}

func (ik *NloptIK) computeLimits(seed, travelPercent []float64) ([]float64, []float64) {
	lowerBound, upperBound := limitsToArrays(ik.limits)

	if len(travelPercent) == len(lowerBound) {
		for i := 0; i < len(lowerBound); i++ {
			lowerBound[i] = max(lowerBound[i], seed[i]-(ik.limits[i].Range()*travelPercent[i]))
			upperBound[i] = min(upperBound[i], seed[i]+(ik.limits[i].Range()*travelPercent[i]))
		}
	}

	return lowerBound, upperBound
}

type nloptSeedState struct {
	seed                   []float64
	lowerBound, upperBound []float64
	jump                   []float64

	meta string

	opt *nlopt.NLopt
}

func (ik *NloptIK) newSeedState(ctx context.Context, seedNumber int, minFunc CostFunc,
	s []float64, travelPercentMultiplier float64, travelPercentIn []float64, iterations *int,
) (*nloptSeedState, error) {
	var err error

	ss := &nloptSeedState{
		seed: s,
		meta: fmt.Sprintf("s:%d-travel:%0.1f", seedNumber, travelPercentMultiplier),
	}

	var travelPercent []float64
	if travelPercentMultiplier <= 0 {
		travelPercent = nil
	} else if travelPercentMultiplier >= 1 {
		travelPercent = travelPercentIn
	} else {
		travelPercent = make([]float64, len(travelPercentIn))
		for i, in := range travelPercentIn {
			travelPercent[i] = min(.5, max(travelPercentMultiplier, in))
		}
	}

	ss.lowerBound, ss.upperBound = ik.computeLimits(s, travelPercent)
	if len(ss.lowerBound) == 0 || len(ss.upperBound) == 0 {
		return nil, errBadBounds
	}

	// Determine optimal jump values; start with default, and if gradient is zero, increase to 1 to try to avoid underflow.
	ss.jump = ik.calcJump(ctx, defaultJump, s, minFunc)

	ss.opt, err = nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ss.lowerBound)))
	if err != nil {
		return nil, errors.Wrap(err, "nlopt creation error")
	}

	err = multierr.Combine(
		ss.opt.SetFtolAbs(defaultGoalThreshold),
		ss.opt.SetLowerBounds(ss.lowerBound),
		ss.opt.SetStopVal(defaultGoalThreshold),
		ss.opt.SetUpperBounds(ss.upperBound),
		ss.opt.SetXtolAbs1(defaultGoalThreshold),
		ss.opt.SetMinObjective(ss.getMinFunc(ctx, minFunc, iterations)),
		ss.opt.SetMaxEval(nloptStepsPerIter),
	)
	if err != nil {
		return nil, err
	}
	if ik.useRelTol {
		err = multierr.Combine(
			ss.opt.SetFtolRel(defaultGoalThreshold),
			ss.opt.SetXtolRel(defaultGoalThreshold),
		)
		if err != nil {
			return nil, err
		}
	}

	return ss, nil
}

func (nss *nloptSeedState) getMinFunc(ctx context.Context, minFunc CostFunc, iteration *int) nlopt.Func {
	// checkVals is our set of inputs that we evaluate for distance
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	return func(checkVals, gradient []float64) float64 {
		*iteration++
		dist := minFunc(ctx, checkVals)
		if len(gradient) > 0 {
			// Yes, the for loop below is logically equivalent to not having this if statement. But CPU branch prediction means having the
			// if statement is faster.
			for i := range gradient {
				jumpVal := nss.jump[i]
				flip := false
				checkVals[i] += jumpVal
				ub := nss.upperBound[i]
				for checkVals[i] >= ub {
					flip = true
					checkVals[i] -= 10 * jumpVal
				}
				dist2 := minFunc(ctx, checkVals)
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
}

// Solve runs the actual solver and sends any solutions found to the given channel.
func (ik *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *Solution,
	seeds [][]float64,
	travelPercent []float64,
	minFunc CostFunc,
	rseed int,
) (int, error) {
	if len(seeds) == 0 {
		return 0, fmt.Errorf("no seeds")
	}

	if len(seeds[0]) != len(ik.limits) {
		return 0, fmt.Errorf("nlopt initialized with %d dof but seed was length %d", len(ik.limits), len(seeds[0]))
	}
	//nolint: gosec
	randSeed := rand.New(rand.NewSource(int64(rseed)))

	seedStates := []*nloptSeedState{}
	defer func() {
		for _, ss := range seedStates {
			if ss.opt != nil {
				ss.opt.Destroy()
			}
		}
	}()

	iterations := 0

	for _, x := range []float64{.1, 1, 0} {
		for i, s := range seeds {
			ss, err := ik.newSeedState(ctx, i, minFunc, s, x, travelPercent, &iterations)
			if err != nil {
				return 0, err
			}

			seedStates = append(seedStates, ss)
		}
	}

	if rseed%3 == 1 {
		slices.Reverse(seedStates)
	}

	solutionsFound := 0
	seedNumber := 0

	for iterations < ik.maxIterations && ctx.Err() == nil {
		iterations++

		ss := seedStates[seedNumber%len(seedStates)]

		solutionRaw, result, nloptErr := ss.opt.Optimize(ss.seed)
		if nloptErr != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			// Above was previous comment.
			// I (Eliot) think this is caused by a bug in how we compute the gradient
			// When the absolute value of the gradient is too high, it blows up
			if nloptErr.Error() != "nlopt: FAILURE" {
				return solutionsFound, nloptErr
			}
		} else if solutionRaw == nil {
			panic("why is solutionRaw nil")
		} else if result < defaultGoalThreshold || !ik.exact {
			solution := &Solution{
				Configuration: solutionRaw,
				Score:         result,
				Exact:         result < defaultGoalThreshold,
				Meta:          ss.meta,
			}
			select {
			case <-ctx.Done():
				break
			case solutionChan <- solution:
				solutionsFound++
			}
		}
		ss.seed = generateRandomPositions(randSeed, ss.lowerBound, ss.upperBound)

		seedNumber++
	}

	return solutionsFound, nil
}

func (ik *NloptIK) calcJump(ctx context.Context, testJump float64, seed []float64, minFunc CostFunc) []float64 {
	jump := make([]float64, 0, len(seed))
	lowerBound, upperBound := limitsToArrays(ik.limits)

	seedDist := minFunc(ctx, seed)
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

			checkDist := minFunc(ctx, seed)

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
