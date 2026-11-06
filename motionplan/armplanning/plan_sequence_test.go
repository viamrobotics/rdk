package armplanning

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func xarm6SequenceRequest(t *testing.T, fs *referenceframe.FrameSystem, goal []float64) *PlanRequest {
	t.Helper()
	req := &PlanRequest{FrameSystem: fs, PlannerOptions: NewBasicPlannerOptions()}
	if goal != nil {
		req.Goals = []*PlanState{NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": goal})}
	}
	return req
}

func xarm6FrameSystem(t *testing.T) *referenceframe.FrameSystem {
	t.Helper()
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)
	return fs
}

// TestPlanMotionSequenceChains checks that every request after the first starts exactly where the
// previous plan ended.
func TestPlanMotionSequenceChains(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := xarm6FrameSystem(t)

	start := []float64{0, 0.5, -1.0, 0, 0.5, 0}
	requests := []*PlanRequest{
		xarm6SequenceRequest(t, fs, []float64{0.2, 0.5, -1.0, 0, 0.5, 0}),
		xarm6SequenceRequest(t, fs, []float64{0.2, 0.3, -0.8, 0, 0.5, 0}),
		xarm6SequenceRequest(t, fs, []float64{-0.1, 0.3, -0.8, 0.1, 0.5, 0}),
	}
	requests[0].StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": start})

	plans, metas, err := PlanMotionSequence(context.Background(), logger, requests)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plans, test.ShouldHaveLength, len(requests))

	for i := 1; i < len(plans); i++ {
		prev := plans[i-1].Trajectory()
		test.That(t, plans[i].Trajectory()[0]["arm"], test.ShouldResemble, prev[len(prev)-1]["arm"])
		test.That(t, metas[i].BacktrackCounter, test.ShouldEqual, 0)
	}
}

// TestPlanMotionSequenceStaticFrameRemoved chains into a request whose frame system no longer has a
// static frame the previous one had, as when a held object is released between requests.
func TestPlanMotionSequenceStaticFrameRemoved(t *testing.T) {
	logger := logging.NewTestLogger(t)

	withFilter := xarm6FrameSystem(t)
	filter, err := referenceframe.NewStaticFrame("filter", spatialmath.NewPoseFromPoint(r3.Vector{X: 300}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, withFilter.AddFrame(filter, withFilter.Frame("arm")), test.ShouldBeNil)

	requests := []*PlanRequest{
		xarm6SequenceRequest(t, withFilter, []float64{0.2, 0.5, -1.0, 0, 0.5, 0}),
		xarm6SequenceRequest(t, xarm6FrameSystem(t), []float64{0.2, 0.3, -0.8, 0, 0.5, 0}),
	}
	requests[0].StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{
		"arm": {0, 0.5, -1.0, 0, 0.5, 0}, "filter": {},
	})

	plans, metas, err := PlanMotionSequence(context.Background(), logger, requests)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, metas[1].BacktrackCounter, test.ShouldEqual, 0)
	test.That(t, requests[1].StartState.Configuration(), test.ShouldNotContainKey, "filter")
	test.That(t, plans[1].Trajectory()[0]["arm"], test.ShouldResemble, []float64{0.2, 0.5, -1.0, 0, 0.5, 0})
}

func TestCarryOverConfigurationNeedsEveryMovingFrame(t *testing.T) {
	_, err := carryOverConfiguration(referenceframe.FrameSystemInputs{}, xarm6FrameSystem(t))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `frame "arm" has 6 DoF`)
}

// TestPlanMotionSequenceStopsWhenContextDone checks that a request which fails because the context is
// done ends the sequence, rather than being taken as a reason to backtrack.
func TestPlanMotionSequenceStopsWhenContextDone(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := xarm6FrameSystem(t)

	requests := []*PlanRequest{
		xarm6SequenceRequest(t, fs, []float64{0.2, 0.5, -1.0, 0, 0.5, 0}),
		// Unplannable, so without the context check this would backtrack 8 times.
		xarm6SequenceRequest(t, fs, nil),
	}
	requests[0].StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": {0, 0.5, -1.0, 0, 0.5, 0}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, metas, err := PlanMotionSequence(ctx, logger, requests)
	test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "sequence stopped at plan 1/2")
	test.That(t, metas[0], test.ShouldBeNil)
}

// TestPlanMotionSequenceBacktracksToFirstRequest makes the last request unplannable from any start,
// which forces the search to give up on start states two requests back before it fails. Every start
// state of a request gets backtrackBranchFactor (3) attempts, so request 0 is planned 3 times,
// request 1 3 times per request 0 plan, and request 2 once per request 1 plan.
func TestPlanMotionSequenceBacktracksToFirstRequest(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fs := xarm6FrameSystem(t)

	firstStart := NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": {0, 0.5, -1.0, 0, 0.5, 0}})
	requests := []*PlanRequest{
		xarm6SequenceRequest(t, fs, []float64{0.2, 0.5, -1.0, 0, 0.5, 0}),
		xarm6SequenceRequest(t, fs, []float64{0.2, 0.3, -0.8, 0, 0.5, 0}),
		// No goals, so it fails validation whatever it starts from.
		xarm6SequenceRequest(t, fs, nil),
	}
	requests[0].StartState = firstStart

	_, metas, err := PlanMotionSequence(context.Background(), logger, requests)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Rejected solutions before giving up: 3")

	test.That(t, requests[0].StartState, test.ShouldEqual, firstStart)
	test.That(t, metas[0].BacktrackCounter, test.ShouldEqual, 2)
	test.That(t, metas[1].BacktrackCounter, test.ShouldEqual, 8)
	test.That(t, metas[2].BacktrackCounter, test.ShouldEqual, 8)
}
