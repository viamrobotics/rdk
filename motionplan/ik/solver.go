// Package ik contains tols for doing gradient-descent based inverse kinematics, allowing for the minimization of arbitrary metrics
// based on the output of calling `Transform` on the given frame.
package ik

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync/atomic"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
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

// SeedSolveMetaData meta data about how a seed did
type SeedSolveMetaData struct {
	Attempts int
	Errors   int
	Valid    int
}

// Solver defines an interface which, provided with seed inputs and a function to minimize to zero, will output all found
// solutions to the provided channel until cancelled or otherwise completes.
type Solver interface {
	// Solve receives a context, a channel to which solutions will be provided, a function whose output should be minimized, and a
	// number of iterations to run.
	Solve(ctx context.Context, solutions chan<- *Solution, totalAttempts *atomic.Int32,
		seeds [][]float64, limits [][]referenceframe.Limit,
		minFunc CostFunc, rseed int) (int, []SeedSolveMetaData, error)
}

// Solution is the struct returned from an IK solver. It contains the solution configuration, the score of the solution, and a flag
// indicating whether that configuration and score met the solution criteria requested by the caller.
type Solution struct {
	Configuration []float64
	Score         float64
	Exact         bool
	Meta          string
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

func findFixedJoints(lowerBound, upperBound []float64) ([]int, []float64) {
	var indices []int
	var values []float64
	for i, l := range lowerBound {
		if l == upperBound[i] {
			indices = append(indices, i)
			values = append(values, l)
		}
	}
	return indices, values
}

func removeFixedIndices(seed, lowerBound, upperBound []float64, fixedIndices []int) ([]float64, []float64, []float64) {
	if len(fixedIndices) == 0 {
		return seed, lowerBound, upperBound
	}
	reducedLen := len(seed) - len(fixedIndices)
	reducedSeed := make([]float64, 0, reducedLen)
	reducedLower := make([]float64, 0, reducedLen)
	reducedUpper := make([]float64, 0, reducedLen)
	fixedIdx := 0
	for i := range seed {
		if fixedIdx < len(fixedIndices) && fixedIndices[fixedIdx] == i {
			fixedIdx++
			continue
		}
		reducedSeed = append(reducedSeed, seed[i])
		reducedLower = append(reducedLower, lowerBound[i])
		reducedUpper = append(reducedUpper, upperBound[i])
	}
	return reducedSeed, reducedLower, reducedUpper
}

func removeAtIndices(vals []float64, indices []int) []float64 {
	if len(indices) == 0 {
		return vals
	}
	result := make([]float64, 0, len(vals)-len(indices))
	j := 0
	for i, v := range vals {
		if j < len(indices) && indices[j] == i {
			j++
			continue
		}
		result = append(result, v)
	}
	return result
}

func insertFixedJoints(reduced []float64, fixedIndices []int, fixedValues []float64) []float64 {
	if len(fixedIndices) == 0 {
		return reduced
	}
	full := make([]float64, len(reduced)+len(fixedIndices))
	fixedIdx := 0
	reducedIdx := 0
	for i := range full {
		if fixedIdx < len(fixedIndices) && fixedIndices[fixedIdx] == i {
			full[i] = fixedValues[fixedIdx]
			fixedIdx++
		} else {
			full[i] = reduced[reducedIdx]
			reducedIdx++
		}
	}
	return full
}

// DoSolve is a synchronous wrapper around Solver.Solve.
// rangeModifier is [0-1] - 0 means don't really look a lot, which is good for highly constrained things
//
//	but will fail if you have to move. 1 means search the entire range.
func DoSolve(ctx context.Context, solver Solver, totalAttempts *atomic.Int32, solveFunc CostFunc,
	seeds [][]float64, limits [][]referenceframe.Limit,
) ([][]float64, []SeedSolveMetaData, error) {
	limits, err := fixLimits(len(seeds), limits)
	if err != nil {
		return nil, nil, err
	}

	solutionGen := make(chan *Solution)

	var solveErrors error
	var meta []SeedSolveMetaData

	go func() {
		defer close(solutionGen)
		_, m, err := solver.Solve(ctx, solutionGen, totalAttempts, seeds, limits, solveFunc, 1)
		solveErrors = err
		meta = m
	}()

	var solutions [][]float64
	for step := range solutionGen {
		solutions = append(solutions, step.Configuration)
	}

	if solveErrors != nil {
		return nil, nil, solveErrors
	}

	if len(solutions) == 0 {
		return nil, nil, fmt.Errorf("unable to solve for position")
	}

	return solutions, meta, nil
}

func fixLimits(numSeeds int, limits [][]referenceframe.Limit) ([][]referenceframe.Limit, error) {
	if numSeeds == len(limits) {
		return limits, nil
	}

	if len(limits) == 0 {
		return nil, fmt.Errorf("have no limits")
	}

	if len(limits) > 1 {
		return nil, fmt.Errorf("if not specifying limit for every seed, can only specify 1, not %d", len(limits))
	}

	newLimits := [][]referenceframe.Limit{}

	for range numSeeds {
		newLimits = append(newLimits, limits[0])
	}

	return newLimits, nil
}

// ComputeAdjustLimits adjusts limits by delta.
func ComputeAdjustLimits(seed []float64, limits []referenceframe.Limit, delta float64) []referenceframe.Limit {
	if delta <= 0 || delta >= 1 {
		return limits
	}

	newLimits := []referenceframe.Limit{}

	for i, s := range seed {
		lmin, lmax, r := limits[i].GoodLimits()
		d := r * delta

		newLimits = append(newLimits, referenceframe.Limit{max(lmin, s-d), min(lmax, s+d)})
	}
	return newLimits
}

// ComputeAdjustLimitsArray adjusts limits by deltas for each limit
func ComputeAdjustLimitsArray(seed []float64, limits []referenceframe.Limit, deltas []float64) []referenceframe.Limit {
	if len(limits) != len(seed) || len(deltas) != len(seed) {
		panic(fmt.Errorf("bad args seed: %d limits: %d deltas: %d", len(seed), len(limits), len(deltas)))
	}
	newLimits := []referenceframe.Limit{}

	for i, s := range seed {
		lmin, lmax, r := limits[i].GoodLimits()
		d := r * deltas[i]

		newLimits = append(newLimits, referenceframe.Limit{max(lmin, s-d), min(lmax, s+d)})
	}
	return newLimits
}

// NewMetricMinFunc creates a cost function that minimizes distance to a goal pose using the specified metric
func NewMetricMinFunc(metricFunc func(spatialmath.Pose) float64, frame referenceframe.Frame, logger logging.Logger) CostFunc {
	return func(ctx context.Context, inputs []float64) float64 {
		currentPose, err := frame.Transform(inputs)
		if err != nil {
			logger.Debugf("Transform error in metric: %v", err)
			return math.Inf(1)
		}
		return metricFunc(currentPose)
	}
}
