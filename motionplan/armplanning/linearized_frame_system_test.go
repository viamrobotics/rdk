package armplanning

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestLinearFrameSystemGoalThing(t *testing.T) {
	logger := logging.NewTestLogger(t)

	req, err := readRequestFromFile("data/orb-plan1.json")
	test.That(t, err, test.ShouldBeNil)

	lfs, err := newLinearizedFrameSystem(req.FrameSystem)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(lfs.frames), test.ShouldEqual, 8)
	test.That(t, len(lfs.dof), test.ShouldEqual, 6)

	mc, err := motionChainsFromPlanState(req.FrameSystem, req.Goals[0])
	test.That(t, err, test.ShouldBeNil)

	toChange := lfs.inputChangeRatio(
		mc,
		req.StartState.configuration,
		req.PlannerOptions.getGoalMetric(req.Goals[0].poses),
		logger)

	test.That(t, len(toChange), test.ShouldEqual, 6)

	test.That(t, toChange[0], test.ShouldAlmostEqual, minJogPercent, .01)
	test.That(t, toChange[1], test.ShouldAlmostEqual, minJogPercent, .01)
	test.That(t, toChange[2], test.ShouldAlmostEqual, minJogPercent, .01)
	test.That(t, toChange[3], test.ShouldAlmostEqual, minJogPercent, .01)
	test.That(t, toChange[4], test.ShouldAlmostEqual, minJogPercent, .01)

	test.That(t, toChange[5], test.ShouldBeGreaterThan, minJogPercent)
	test.That(t, toChange[5], test.ShouldBeLessThan, .25)
}
