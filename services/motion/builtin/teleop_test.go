package builtin

import (
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

func TestTeleopPlanJump(t *testing.T) {
	rads := func(degs ...float64) []referenceframe.Input {
		out := make([]referenceframe.Input, len(degs))
		for i, d := range degs {
			out[i] = utils.DegToRad(d)
		}
		return out
	}
	arms := map[string]bool{"arm": true}
	head := referenceframe.FrameSystemInputs{"arm": rads(42.9, -34.0, 8.0, -2.5, 20.9, 42.9), "gripper": rads(0)}

	// An ordinary step, a few degrees after 10ms, is accepted.
	next := referenceframe.FrameSystemInputs{"arm": rads(43.3, -34.4, 8.2, -2.4, 21.1, 43.2), "gripper": rads(0)}
	test.That(t, planJump(head, next, arms, 10*time.Millisecond).tooFar, test.ShouldBeFalse)

	// A flip to another kinematic branch, as seen on an xArm6 for a 1mm hand move 176ms after
	// the previous plan, is refused.
	flipped := referenceframe.FrameSystemInputs{"arm": rads(45.7, -1.7, -22.3, 176.6, -19.2, -133.5), "gripper": rads(0)}
	res := planJump(head, flipped, arms, 176*time.Millisecond)
	test.That(t, res.tooFar, test.ShouldBeTrue)
	test.That(t, res.arm, test.ShouldEqual, "arm")
	test.That(t, res.degs, test.ShouldAlmostEqual, 179.1, 0.1)
	test.That(t, res.allowedDegs, test.ShouldBeLessThan, 40)

	// It is refused however long ago the last plan was.
	test.That(t, planJump(head, flipped, arms, time.Hour).tooFar, test.ShouldBeTrue)

	// A smaller discontinuity, 22 degrees 21ms after the previous plan, is refused too.
	hop := referenceframe.FrameSystemInputs{"arm": rads(42.9, -56.0, 8.0, -2.5, 20.9, 42.9), "gripper": rads(0)}
	test.That(t, planJump(head, hop, arms, 21*time.Millisecond).tooFar, test.ShouldBeTrue)

	// Catching up after a pause in the operator's poses is allowed: 30 degrees after 300ms.
	catchUp := referenceframe.FrameSystemInputs{"arm": rads(42.9, -4.0, 8.0, -2.5, 20.9, 42.9), "gripper": rads(0)}
	test.That(t, planJump(head, catchUp, arms, 300*time.Millisecond).tooFar, test.ShouldBeFalse)

	// Components that are not arms are not judged.
	gripperOnly := referenceframe.FrameSystemInputs{"arm": head["arm"], "gripper": rads(90)}
	test.That(t, planJump(head, gripperOnly, arms, time.Millisecond).tooFar, test.ShouldBeFalse)
}
