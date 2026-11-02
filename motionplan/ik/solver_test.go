package ik

import (
	"context"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home = [][]float64{{0, 0, 0, 0, 0, 0}}
	nCPU = int(math.Max(1.0, float64(runtime.NumCPU()/4)))
)

func TestCombinedIKinematics(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(logger, nCPU, defaultGoalThreshold, time.Second)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := spatial.NewPose(
		r3.Vector{X: -46, Y: -133, Z: 372},
		&spatial.OrientationVectorDegrees{OX: 1.79, OY: -1.32, OZ: -1.11},
	)
	solveFunc := NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)
	var totalAttempts atomic.Int32
	solution, _, err := DoSolve(context.Background(), ik, &totalAttempts, solveFunc, home, [][]frame.Limit{m.DoF()})
	test.That(t, err, test.ShouldBeNil)

	// Test moving forward 20 in X direction from previous position
	pos = spatial.NewPose(
		r3.Vector{X: -66, Y: -133, Z: 372},
		&spatial.OrientationVectorDegrees{OX: 1.78, OY: -3.3, OZ: -1.11},
	)
	solveFunc = NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)
	_, _, err = DoSolve(context.Background(), ik, &totalAttempts, solveFunc, solution, [][]frame.Limit{m.DoF()})
	test.That(t, err, test.ShouldBeNil)
}

func TestUR5NloptIKinematics(t *testing.T) {
	logger := logging.NewTestLogger(t)

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(logger, nCPU, defaultGoalThreshold, time.Second)
	test.That(t, err, test.ShouldBeNil)

	goalJP := frame.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal, err := m.Transform(m.InputFromProtobuf(goalJP))
	test.That(t, err, test.ShouldBeNil)
	solveFunc := NewMetricMinFunc(motionplan.NewSquaredNormMetric(goal), m, logger)
	var totalAttempts atomic.Int32
	_, _, err = DoSolve(context.Background(), ik, &totalAttempts, solveFunc, home, [][]frame.Limit{m.DoF()})
	test.That(t, err, test.ShouldBeNil)
}

// TestComputeAdjustLimitsArrayStaysWithinInputLimits asserts the adjusted limits never extend
// beyond the limits they were derived from, including when the seed starts out of bounds. Items in
// the input arrays are independent, so each case uses arrays of size one.
func TestComputeAdjustLimitsArrayStaysWithinInputLimits(t *testing.T) {
	// In bounds.
	limits := []frame.Limit{{Min: 0, Max: 1}}
	adjusted := ComputeAdjustLimitsArray([]float64{0.5}, limits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 1)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// In bounds, with the window clipped by the max.
	adjusted = ComputeAdjustLimitsArray([]float64{0.99}, limits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 1)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// Seed far above the max.
	adjusted = ComputeAdjustLimitsArray([]float64{5}, limits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 1)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// Seed far below the min.
	adjusted = ComputeAdjustLimitsArray([]float64{-5}, limits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 1)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// Seed below the min by more than the window. The adjusted limits must stay ordered, not just
	// within the input limits. This mirrors a shoulder lift with an override of +-pi/4 that was moved
	// directly to -pi.
	overrideLimits := []frame.Limit{{Min: -math.Pi / 4, Max: math.Pi / 4}}
	adjusted = ComputeAdjustLimitsArray([]float64{-math.Pi}, overrideLimits, []float64{1})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, -math.Pi/4)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, math.Pi/4)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// Zero width limit, seed at the limit.
	zeroWidthLimits := []frame.Limit{{Min: 0, Max: 0}}
	adjusted = ComputeAdjustLimitsArray([]float64{0}, zeroWidthLimits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 0)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// Zero width limit, seed above the limit.
	adjusted = ComputeAdjustLimitsArray([]float64{0.5}, zeroWidthLimits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 0)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)

	// Zero width limit, seed below the limit.
	adjusted = ComputeAdjustLimitsArray([]float64{-0.5}, zeroWidthLimits, []float64{0.05})
	test.That(t, adjusted[0].Min, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, adjusted[0].Max, test.ShouldBeLessThanOrEqualTo, 0)
	test.That(t, adjusted[0].Min, test.ShouldBeLessThanOrEqualTo, adjusted[0].Max)
}
