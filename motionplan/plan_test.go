package motionplan

import (
	"math"
	"testing"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/test"
)

func TestEvaluateTrajectory(t *testing.T) {
	plan := Trajectory{
		map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}},
	}
	score := plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Inf(1))

	// Test no change
	plan = append(plan, map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, 0)

	// Test L2 for "", and nothing for plan with only one entry
	plan = append(plan, map[string][]referenceframe.Input{"": {{4.}, {5.}, {6.}}, "test": {{2.}, {3.}, {4.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27))

	// Test cumulative L2 after returning to original inputs
	plan = append(plan, map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2)

	// Test that the "test" inputs are properly evaluated after skipping a step
	plan = append(plan, map[string][]referenceframe.Input{"test": {{3.}, {5.}, {6.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2+3)

	// Evaluated with the tp-space metric, should be the sum of the distance values (third input) ignoring the first input set for each
	// named input set
	score = plan.Evaluate(tpspace.PTGSegmentMetric)
	test.That(t, score, test.ShouldEqual, 18)
}
