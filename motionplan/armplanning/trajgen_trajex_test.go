//go:build !windows && !no_cgo

package armplanning_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// TestPlanMotionTrajGenUsesTrajex runs a real plan through the trajectory-generator path
// and asserts the result is a time-parameterized TOTG trajectory.
//
// ToTrajGen now unconditionally returns the in-process trajex (cgo) backend, so a successful
// TrajGenPlan whose samples carry monotonic timestamps and per-sample velocities is proof
// that the trajex library performed the generation — plain RRT waypoints carry neither.
func TestPlanMotionTrajGenUsesTrajex(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(t, err, test.ShouldBeNil)

	fs := frame.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	// Config-to-config move (no IK, no obstacles) so planning is deterministic.
	startInputs := frame.FrameSystemInputs{"xarm6": []frame.Input{0, 0, 0, 0, 0, 0}}
	goalInputs := frame.FrameSystemInputs{"xarm6": []frame.Input{0.5, -0.3, -0.4, 0.1, 0.2, -0.1}}

	req := &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, startInputs),
		Goals:       []*armplanning.PlanState{armplanning.NewPlanState(nil, goalInputs)},
	}

	// In-process trajex backend (cgo); 1 rad/s velocity, 2 rad/s^2 acceleration limits.
	trajGen, err := (&armplanning.TrajGenConfig{
		VelocityLimitsRadsPerSec:      1.0,
		AccelerationLimitsRadsPerSec2: 2.0,
	}).ToTrajGen()
	test.That(t, err, test.ShouldBeNil)

	plan, _, err := armplanning.PlanMotionTrajGen(ctx, logger, req, trajGen)
	test.That(t, err, test.ShouldBeNil)

	tgPlan, ok := plan.(*armplanning.TrajGenPlan)
	test.That(t, ok, test.ShouldBeTrue)

	// A TOTG trajectory is densely sampled and time-parameterized: equal-length sample
	// times, per-sample configurations, and per-sample velocities.
	nSamples := len(tgPlan.SampleTimes)
	test.That(t, nSamples, test.ShouldBeGreaterThan, 0)
	test.That(t, len(tgPlan.Configurations), test.ShouldEqual, nSamples)
	test.That(t, len(tgPlan.Velocities), test.ShouldEqual, nSamples)

	// Sample times start at t=0 and strictly increase — the hallmark of the trajex time
	// parameterization, which the raw RRT waypoints do not carry.
	test.That(t, tgPlan.SampleTimes[0], test.ShouldEqual, 0.0)
	for i := 1; i < nSamples; i++ {
		test.That(t, tgPlan.SampleTimes[i], test.ShouldBeGreaterThan, tgPlan.SampleTimes[i-1])
	}
}
