package armplanning

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

func TestWineCrazyTouch3(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-two-arm.json")
	test.That(t, err, test.ShouldBeNil)

	req.myTestOptions.doNotCloseObstacles = true

	plan, meta, err := PlanMotion(ctx, logger, req)
	test.That(t, err, test.ShouldBeNil)

	orig := plan.Trajectory()[0]["right-arm"]
	for _, tt := range plan.Trajectory() {
		now := tt["right-arm"]
		logger.Infof("r: %v", now)
		logger.Infof("l: %v", tt["left-arm"])
		d := referenceframe.InputsL2Distance(orig, now)
		test.That(t, d, test.ShouldBeLessThan, 0.0001)
	}

	test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 6)

	// ---

	pc, err := newPlanContext(ctx, logger, req, meta)
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(t, err, test.ShouldBeNil)

	// Convert trajectory to steps
	steps := make([]*referenceframe.LinearInputs, 0, len(plan.Trajectory()))
	for _, step := range plan.Trajectory() {
		steps = append(steps, step.ToLinearInputs())
	}

	// Adjust right-arm values a bit to simulate unnecessary motion
	// This tests that tryOnlyMovingComponentsThatNeedToMove will fix them
	origRightArm := steps[0].Get("right-arm")
	for i := 1; i < len(steps); i++ {
		// Modify right-arm slightly (add small perturbation)
		rightArm := steps[i].Get("right-arm")
		modified := make([]float64, len(rightArm))
		for j := range rightArm {
			modified[j] = rightArm[j] + 0.001 // Add small perturbation
		}
		steps[i].Put("right-arm", modified)
	}

	// Verify that right-arm has been modified
	for i := 1; i < len(steps); i++ {
		d := referenceframe.InputsL2Distance(origRightArm, steps[i].Get("right-arm"))
		test.That(t, d, test.ShouldBeGreaterThan, 0.0001) // Should be perturbed now
	}

	// Run tryOnlyMovingComponentsThatNeedToMove - this should fix the right-arm back
	steps = tryOnlyMovingComponentsThatNeedToMove(ctx, psc, steps)

	// Verify that right-arm is back to original (within tolerance)
	for i := 1; i < len(steps); i++ {
		d := referenceframe.InputsL2Distance(origRightArm, steps[i].Get("right-arm"))
		test.That(t, d, test.ShouldBeLessThan, 0.0001) // Should be fixed back
	}
}
