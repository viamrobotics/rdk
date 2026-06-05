//go:build !windows && !no_cgo

// Package nloptik provides an nlopt-backed implementation of ik.Solver.
// Blank-import this package (`import _ "go.viam.com/rdk/motionplan/ik/nloptik"`)
// in any binary or test that needs gradient-descent inverse kinematics.
package nloptik

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/go-nlopt/nlopt"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

func init() {
	ik.RegisterDefaultSolver(func(
		logger logging.Logger,
		iter int,
		exact, useRelTol bool,
		maxTime time.Duration,
	) (ik.Solver, error) {
		return CreateNloptSolver(logger, iter, exact, useRelTol, maxTime)
	})
}

var errBadBounds = errors.New("cannot set upper or lower bounds for nlopt, slice is empty. Are you trying to move a static frame?")

// NloptAlg is what algorith to use - nlopt.LD_SLSQP is the original one we used
var NloptAlg = nlopt.LD_SLSQP

const (
	nloptStepsPerIter = 4001
	defaultMaxIter    = 5000
	defaultJump       = 1e-8

	// matches ik.defaultGoalThreshold (kept local so nloptik stays self-contained).
	defaultGoalThreshold = 0.001 * 0.001

	// matches ik.defaultLimitSeedPoint.
	defaultLimitSeedPoint = 999
)

// NloptIK can solve IK problems with nlopt.
type NloptIK struct {
	maxIterations int
	logger        logging.Logger

	// Nlopt will try to minimize a configuration for whatever is passed in. If exact is false, then the solver will emit partial
	// solutions where it was not able to meet the goal criteria but still was able to improve upon the seed.
	exact bool

	// useRelTol specifies whether the SetXtolRel and SetFtolRel values will be set for nlopt.
	// If true, this will terminate solving when nlopt alg iterations change the distance to goal by less than some proportion of calculated
	// distance. This can cause premature terminations when the distances are large.
	useRelTol bool

	maxTime time.Duration
}

// CreateNloptSolver creates an nloptIK object that can perform gradient descent on functions. The parameters are the limits
// of the solver, a logger, and the number of iterations to run. If the iteration count is less than 1, it will be set
// to the default of 5000.
func CreateNloptSolver(
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
	maxTime time.Duration,
) (*NloptIK, error) {
	s := &NloptIK{logger: logger}

	if iter < 1 {
		// default value
		iter = defaultMaxIter
	}
	s.maxIterations = iter
	s.exact = exact
	s.useRelTol = useRelTol
	s.maxTime = maxTime

	return s, nil
}

type nloptSeedState struct {
	seed                   []float64
	lowerBound, upperBound []float64
	jump                   []float64

	meta string

	opt    *nlopt.NLopt
	logger logging.Logger
}

func (s *NloptIK) newSeedState(ctx context.Context, seedNumber int, minFunc ik.CostFunc,
	seed []float64, limits []referenceframe.Limit, iterations *int,
) (*nloptSeedState, error) {
	var err error

	ss := &nloptSeedState{
		seed:   seed,
		meta:   fmt.Sprintf("s:%d", seedNumber),
		logger: s.logger,
	}

	ss.lowerBound, ss.upperBound = limitsToArrays(limits)
	if len(ss.lowerBound) == 0 || len(ss.upperBound) == 0 {
		return nil, errBadBounds
	}

	// nlopt returns INVALID_ARGS for zero-range variables - nudge the upper bound by a small epsilon.
	for i := range ss.lowerBound {
		if ss.lowerBound[i] == ss.upperBound[i] {
			ss.upperBound[i] += defaultGoalThreshold
		}
	}

	// Determine optimal jump values; start with default, and if gradient is zero, increase to 1 to try to avoid underflow.
	ss.jump = s.calcJump(ctx, defaultJump, seed, limits, minFunc)
	ss.opt, err = nlopt.NewNLopt(NloptAlg, uint(len(ss.lowerBound)))
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
	if s.useRelTol {
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

func (nss *nloptSeedState) getMinFunc(ctx context.Context, minFunc ik.CostFunc, iteration *int) nlopt.Func {
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
		nss.logger.Debugf("\t minfunc seed:%s vals: %v dist: %0.2f gradient: %v",
			nss.meta, logging.FloatArrayFormat{"%0.5f", checkVals}, dist, logging.FloatArrayFormat{"", gradient})
		return dist
	}
}

// Solve runs the actual solver and sends any solutions found to the given channel.
func (s *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *ik.Solution,
	totalAttempts *atomic.Int32,
	seeds [][]float64,
	limits [][]referenceframe.Limit,
	minFunc ik.CostFunc,
	rseed int,
) (int, []ik.SeedSolveMetaData, error) {
	if len(seeds) == 0 {
		return 0, nil, fmt.Errorf("no seeds")
	}

	if len(seeds) != len(limits) {
		return 0, nil, fmt.Errorf("need matching limits (%d) and seeds (%d) arrays", len(limits), len(seeds))
	}

	randSeed := rand.New(rand.NewSource(int64(rseed))) //nolint: gosec

	seedStates := []*nloptSeedState{}
	defer func() {
		for _, ss := range seedStates {
			if ss.opt != nil {
				ss.opt.Destroy()
			}
		}
	}()

	meta := []ik.SeedSolveMetaData{}

	iterations := 0

	for i, seed := range seeds {
		ss, err := s.newSeedState(ctx, i, minFunc, seed, limits[i], &iterations)
		if err != nil {
			return 0, nil, err
		}

		seedStates = append(seedStates, ss)
		meta = append(meta, ik.SeedSolveMetaData{})
	}

	solutionsFound := 0
	seedNumber := rseed // start randomly in the list

	itStart := time.Now()
	// maxIterations < 10 opts out of the time-based extension, running to exactly that many
	// iterations regardless of machine speed. This ensures deterministic behavior on slow or
	// busy machines.
	for (iterations < s.maxIterations || (s.maxIterations >= 10 && time.Since(itStart) < s.maxTime)) && ctx.Err() == nil {
		iterations++

		seedNumberRanged := seedNumber % len(seedStates)
		ss := seedStates[seedNumberRanged]
		meta[seedNumberRanged].Attempts++
		if totalAttempts != nil {
			totalAttempts.Add(1)
		}

		solutionRaw, result, nloptErr := ss.opt.Optimize(ss.seed)
		s.logger.Debugf("seed (%d) %v\n\t result: %0.2f  err: %v res: %v",
			seedNumberRanged, logging.FloatArrayFormat{"", ss.seed},
			result, nloptErr, logging.FloatArrayFormat{"", solutionRaw})

		if nloptErr != nil {
			meta[seedNumberRanged].Errors++
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			// Above was previous comment.
			// I (Eliot) think this is caused by a bug in how we compute the gradient
			// When the absolute value of the gradient is too high, it blows up
			if nloptErr.Error() != "nlopt: FAILURE" {
				return solutionsFound, nil, nloptErr
			}
		} else if solutionRaw == nil {
			panic("why is solutionRaw nil")
		} else if result < defaultGoalThreshold || !s.exact {
			meta[seedNumberRanged].Valid++
			solution := &ik.Solution{
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

	return solutionsFound, meta, nil
}

func (s *NloptIK) calcJump(ctx context.Context, testJump float64,
	seed []float64, limits []referenceframe.Limit, minFunc ik.CostFunc,
) []float64 {
	jump := make([]float64, 0, len(seed))
	lowerBound, upperBound := limitsToArrays(limits)

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

// limitsToArrays mirrors the unexported helper in package ik.
func limitsToArrays(limits []referenceframe.Limit) ([]float64, []float64) {
	//nolint: revive
	var min, max []float64
	for _, limit := range limits {
		//nolint: revive
		min = append(min, limit.Min)
		//nolint: revive
		max = append(max, limit.Max)
	}
	return min, max
}

// generateRandomPositions mirrors the unexported helper in package ik.
func generateRandomPositions(randSeed *rand.Rand, lowerBound, upperBound []float64) []float64 {
	pos := make([]float64, len(lowerBound))
	for i, l := range lowerBound {
		u := upperBound[i]

		if l == math.Inf(-1) {
			l = -defaultLimitSeedPoint
		}
		if u == math.Inf(1) {
			u = defaultLimitSeedPoint
		}

		jRange := math.Abs(u - l)
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		pos[i] = randSeed.Float64()*jRange + l
	}
	return pos
}
