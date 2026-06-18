//go:build !windows && !no_cgo

package armplanning_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// twoArmRequest builds a frame system with two xarm6 arms (1m apart) and a plan request that
// moves only arm1. The trajectory spans both arms' DOF (12 total).
func twoArmRequest(t *testing.T) *armplanning.PlanRequest {
	t.Helper()
	fs := frame.NewEmptyFrameSystem("")
	arm1, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "arm1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(arm1, fs.World()), test.ShouldBeNil)

	offset, err := frame.NewStaticFrame("arm2_base", spatialmath.NewPoseFromPoint(r3.Vector{X: 1000}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(offset, fs.World()), test.ShouldBeNil)
	arm2, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "arm2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(arm2, offset), test.ShouldBeNil)

	startInputs := frame.FrameSystemInputs{
		"arm1": []frame.Input{0, 0, 0, 0, 0, 0},
		"arm2": []frame.Input{0, 0, 0, 0, 0, 0},
	}
	goalInputs := frame.FrameSystemInputs{
		"arm1": []frame.Input{0.5, -0.3, -0.4, 0.1, 0.2, -0.1},
		"arm2": []frame.Input{0, 0, 0, 0, 0, 0},
	}
	return &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, startInputs),
		Goals:       []*armplanning.PlanState{armplanning.NewPlanState(nil, goalInputs)},
	}
}

// uniformLimits returns a length-n slice with every entry set to v.
func uniformLimits(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// TestPlanMotionTrajGenMultiArm verifies trajectory generation works with two arms when the
// config provides one limit per joint across both arms' DOF.
func TestPlanMotionTrajGenMultiArm(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	req := twoArmRequest(t)

	// Two xarm6 arms -> 12 DOF. Uniform limits, so DOF ordering is irrelevant.
	trajGen, err := (&armplanning.TrajGenConfig{
		VelocityLimitsRadsPerSec:      uniformLimits(12, 1.0),
		AccelerationLimitsRadsPerSec2: uniformLimits(12, 2.0),
	}).ToTrajGen()
	test.That(t, err, test.ShouldBeNil)

	req.TrajGen = trajGen
	plan, _, err := armplanning.PlanMotion(ctx, logger, req)
	test.That(t, err, test.ShouldBeNil)
	tgPlan, ok := plan.(*armplanning.TrajGenPlan)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(tgPlan.SampleTimes), test.ShouldBeGreaterThan, 0)
}

// TestPlanMotionTrajGenSkipsUnconfigured verifies that when the config doesn't cover the
// trajectory's DOF, trajectory generation is skipped and the planned path is returned as a
// plain plan (not a TrajGenPlan) without error.
func TestPlanMotionTrajGenSkipsUnconfigured(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	req := twoArmRequest(t)

	// Only 6 limits for a 12-DOF trajectory -> doesn't cover the second arm -> skip.
	trajGen, err := (&armplanning.TrajGenConfig{
		VelocityLimitsRadsPerSec:      uniformLimits(6, 1.0),
		AccelerationLimitsRadsPerSec2: uniformLimits(6, 2.0),
	}).ToTrajGen()
	test.That(t, err, test.ShouldBeNil)

	req.TrajGen = trajGen
	plan, _, err := armplanning.PlanMotion(ctx, logger, req)
	test.That(t, err, test.ShouldBeNil)
	// Fell back to the planned path: not a TrajGenPlan, but still a usable trajectory.
	_, isTrajGen := plan.(*armplanning.TrajGenPlan)
	test.That(t, isTrajGen, test.ShouldBeFalse)
	test.That(t, len(plan.Trajectory()), test.ShouldBeGreaterThan, 0)
}
