// Package ik contains tols for doing gradient-descent based inverse kinematics, allowing for the minimization of arbitrary metrics
// based on the output of calling `Transform` on the given frame.
package ik

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

const (
	// If limits are infinite, we use this to bound creating random seeds.
	defaultLimitSeedPoint = 999

	// Default distance below which two distances are considered equal.
	defaultEpsilon = 0.001

	// default amount of closeness to get to the goal.
	defaultGoalThreshold = defaultEpsilon * defaultEpsilon
)

// CostFunc is the function to minimize.
type CostFunc func(context.Context, []float64) float64

// Solver defines an interface which, provided with seed inputs and a function to minimize to zero, will output all found
// solutions to the provided channel until cancelled or otherwise completes.
type Solver interface {
	referenceframe.Limited
	// Solve receives a context, a channel to which solutions will be provided, a function whose output should be minimized, and a
	// number of iterations to run.
	Solve(ctx context.Context, solutions chan<- *Solution, seed []float64,
		travelPercent []float64, minFunc CostFunc, rseed int) (int, error)
}

// Solution is the struct returned from an IK solver. It contains the solution configuration, the score of the solution, and a flag
// indicating whether that configuration and score met the solution criteria requested by the caller.
type Solution struct {
	Configuration []float64
	Score         float64
	Exact         bool
}

// generateRandomPositions generates a random set of positions within the limits of this solver.
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

func limitsToArrays(limits []referenceframe.Limit) ([]float64, []float64) {
	var min, max []float64
	for _, limit := range limits {
		min = append(min, limit.Min)
		max = append(max, limit.Max)
	}
	return min, max
}

// NewMetricMinFunc takes a metric and a frame, and converts to a function able to be minimized with Solve().
func NewMetricMinFunc(metric motionplan.StateMetric, frame referenceframe.Frame, logger logging.Logger) CostFunc {
	return func(_ context.Context, x []float64) float64 {
		mInput := &motionplan.State{Frame: frame}
		inputs := referenceframe.FloatsToInputs(x)
		eePos, err := frame.Transform(inputs)
		if eePos == nil || (err != nil && !strings.Contains(err.Error(), referenceframe.OOBErrString)) {
			logger.Errorw("error calculating frame Transform in IK", "error", err)
			return math.Inf(1)
		}
		mInput.Configuration = inputs
		mInput.Position = eePos
		return metric(mInput)
	}
}

// DoSolve is a synchronous wrapper around Solver.Solve.
// rangeModifier is [0-1] - 0 means don't really look a lot, which is good for highly constrained things
//
//	but will fail if you have to move. 1 means search the entire range.
func DoSolve(ctx context.Context, solver Solver, solveFunc CostFunc,
	seed []float64, rangeModifier float64,
) ([][]float64, error) {
	travelPercent := []float64{}
	for range seed {
		travelPercent = append(travelPercent, rangeModifier)
	}

	solutionGen := make(chan *Solution)

	var solveErrors error

	go func() {
		defer close(solutionGen)
		_, err := solver.Solve(ctx, solutionGen, seed, travelPercent, solveFunc, 1)
		solveErrors = err
	}()

	var solutions [][]float64
	for step := range solutionGen {
		solutions = append(solutions, step.Configuration)
	}

	if solveErrors != nil {
		return nil, solveErrors
	}

	if len(solutions) == 0 {
		return nil, fmt.Errorf("unable to solve for position")
	}

	return solutions, nil
}
