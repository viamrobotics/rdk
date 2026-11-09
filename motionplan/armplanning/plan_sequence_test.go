package armplanning

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// unplannableThirdRequest is a three-request xarm6 sequence whose last request has no goals, so it
// fails validation from any start and planning backtracks all the way to the first request.
func unplannableThirdRequest(t *testing.T) []*PlanRequest {
	t.Helper()
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	jointGoal := func(goal []float64) *PlanRequest {
		req := &PlanRequest{FrameSystem: fs, PlannerOptions: NewBasicPlannerOptions()}
		if goal != nil {
			req.Goals = []*PlanState{NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": goal})}
		}
		return req
	}
	requests := []*PlanRequest{
		jointGoal([]float64{0.2, 0.5, -1.0, 0, 0.5, 0}),
		jointGoal([]float64{0.2, 0.3, -0.8, 0, 0.5, 0}),
		jointGoal(nil),
	}
	requests[0].StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": {0, 0.5, -1.0, 0, 0.5, 0}})
	return requests
}

// TestPlanMotionSequenceBacktrackBranchFactor checks how many endings each start state gets. With a
// branch factor of n, request 0 is planned n times, request 1 n times per request 0 plan, and
// request 2 once per request 1 plan.
func TestPlanMotionSequenceBacktrackBranchFactor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		opts     []SequenceOption
		expected [3]int
	}{
		{name: "default", expected: [3]int{2, 8, 8}},
		{name: "five", opts: []SequenceOption{WithBacktrackBranchFactor(5)}, expected: [3]int{4, 24, 24}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, metas, err := PlanMotionSequence(context.Background(), logging.NewTestLogger(t), unplannableThirdRequest(t),
				tc.opts...)
			test.That(t, err, test.ShouldNotBeNil)
			for i, backtracks := range tc.expected {
				test.That(t, metas[i].BacktrackCounter, test.ShouldEqual, backtracks)
			}
		})
	}
}
