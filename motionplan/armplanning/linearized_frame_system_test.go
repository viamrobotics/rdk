package armplanning

import (
	"testing"

	"go.viam.com/rdk/logging"
	"go.viam.com/test"
)

func TestLinearFrameSystemGoalThing(t *testing.T) {
	logger := logging.NewTestLogger(t)
	
	req, err := readRequestFromFile("data/wine-crazy-touch.json")
	test.That(t, err, test.ShouldBeNil)

	lfs, err := newLinearizedFrameSystem(req.FrameSystem)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(lfs.frames), test.ShouldEqual, 28)
	test.That(t, len(lfs.dof), test.ShouldEqual, 12)

	test.That(t, lfs.frames[0].Name(), test.ShouldEqual, "arm-left")
	test.That(t, lfs.frames[2].Name(), test.ShouldEqual, "arm-right")


	mc, err := motionChainsFromPlanState(req.FrameSystem, req.Goals[0])
	test.That(t, err, test.ShouldBeNil)
	
	toChange, err := lfs.inputChangeRatio(
		mc,
		req.StartState.configuration,
		req.Goals[0].poses,
		req.PlannerOptions.getGoalMetric(req.Goals[0].poses),
		logger)
	test.That(t, err, test.ShouldBeNil)
	
	test.That(t, len(toChange), test.ShouldEqual, 12)

	test.That(t, .1, test.ShouldAlmostEqual, toChange[0], .01)
	test.That(t, .1, test.ShouldAlmostEqual, toChange[5], .01)
	test.That(t, 0, test.ShouldEqual, toChange[6])
	test.That(t, 0, test.ShouldEqual, toChange[11])
	
}
