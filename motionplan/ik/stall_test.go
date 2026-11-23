package ik

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// distanceTo is a cost whose minimum over limits is how far target lies outside them, squared.
func distanceTo(target []float64) CostFunc {
	return func(_ context.Context, inputs []float64) float64 {
		total := 0.0
		for i, v := range inputs {
			d := v - target[i]
			total += d * d
		}
		return total
	}
}

func solveOnce(t *testing.T, logger logging.Logger, target []float64, maxTime time.Duration) (int, []SeedSolveMetaData, time.Duration) {
	t.Helper()
	solver, err := CreateNloptSolver(logger, -1, true, true, maxTime)
	test.That(t, err, test.ShouldBeNil)

	limits := []referenceframe.Limit{{Min: -1, Max: 1}, {Min: -1, Max: 1}}
	solutions := make(chan *Solution)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range solutions {
		}
	}()
	var attempts atomic.Int32
	start := time.Now()
	n, meta, err := solver.Solve(context.Background(), solutions, &attempts,
		[][]float64{{0, 0}}, [][]referenceframe.Limit{limits}, distanceTo(target), 1)
	elapsed := time.Since(start)
	close(solutions)
	<-drained
	test.That(t, err, test.ShouldBeNil)
	return n, meta, elapsed
}

func TestExactSolverGivesUpOnUnreachableGoal(t *testing.T) {
	// The target sits outside the limits, so every descent lands on the same nonzero floor.
	//
	// Every descent logs debug lines, which would flood the output.
	logger := logging.NewTestLogger(t)
	logger.SetLevel(logging.INFO)

	n, meta, elapsed := solveOnce(t, logger, []float64{5, 0}, time.Minute)
	test.That(t, n, test.ShouldEqual, 0)
	test.That(t, meta[0].Attempts, test.ShouldBeGreaterThanOrEqualTo, stallAttempts)
	test.That(t, meta[0].Attempts, test.ShouldBeLessThan, 2*stallAttempts)
	test.That(t, elapsed, test.ShouldBeLessThan, 10*time.Second)
}

func TestExactSolverKeepsSearchingOnceSolved(t *testing.T) {
	// Every descent reaches a reachable target and none improves on the last, so the solver must
	// not mistake that plateau for a stall and must run its full budget.
	maxTime := 200 * time.Millisecond

	// The solver keeps descending for the whole budget, so its debug lines would flood the output.
	logger := logging.NewTestLogger(t)
	logger.SetLevel(logging.INFO)

	n, meta, elapsed := solveOnce(t, logger, []float64{0.5, -0.5}, maxTime)
	test.That(t, n, test.ShouldBeGreaterThan, stallAttempts)
	test.That(t, meta[0].Attempts, test.ShouldBeGreaterThan, stallAttempts)
	test.That(t, elapsed, test.ShouldBeGreaterThanOrEqualTo, maxTime)
}
