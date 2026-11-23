package mpserver

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
)

func TestBuildSequenceBacktrackWithoutRejections(t *testing.T) {
	backtrack := buildSequenceBacktrack(&armplanning.PlanRequest{}, nil, &armplanning.PlanMeta{}, true)
	test.That(t, backtrack, test.ShouldBeNil)
}

func TestBuildSequenceBacktrackCostsEachRejectedSolution(t *testing.T) {
	meta := &armplanning.PlanMeta{RejectedSolutions: []armplanning.RejectedSolution{
		{
			Start: referenceframe.FrameSystemInputs{"arm": {0, 0}},
			End:   referenceframe.FrameSystemInputs{"arm": {3, 4}},
		},
	}}

	backtrack := buildSequenceBacktrack(&armplanning.PlanRequest{}, nil, meta, false)
	test.That(t, backtrack, test.ShouldNotBeNil)

	test.That(t, backtrack.Columns, test.ShouldResemble, []string{"start", "last successful", "rejected 1"})
	test.That(t, len(backtrack.Frames), test.ShouldEqual, 1)
	test.That(t, backtrack.Frames[0].Name, test.ShouldEqual, "arm")
	test.That(t, backtrack.Frames[0].Joints, test.ShouldResemble, [][]sequenceCell{
		{{Value: "-"}, {Value: "-"}, {Value: "3.0000"}},
		{{Value: "-"}, {Value: "-"}, {Value: "4.0000"}},
	})
	test.That(t, backtrack.Frames[0].Cost, test.ShouldResemble, []string{"", "-", "5.0000"})
}

func TestBuildSequenceBacktrackMarksHalfTurnsAndSameJoints(t *testing.T) {
	request := &armplanning.PlanRequest{}
	meta := &armplanning.PlanMeta{RejectedSolutions: []armplanning.RejectedSolution{
		{
			Start: referenceframe.FrameSystemInputs{"arm": {0, 0, 0, 0}},
			End:   referenceframe.FrameSystemInputs{"arm": {1 + math.Pi, 2, 3 + 2*math.Pi, 0.5}},
		},
	}}
	plan := motionplan.NewSimplePlan(nil, motionplan.Trajectory{{"arm": {0, 0, 0, 0}}, {"arm": {1, 2, 3, 1.5}}})

	backtrack := buildSequenceBacktrack(request, plan, meta, true)
	test.That(t, backtrack, test.ShouldNotBeNil)
	test.That(t, backtrack.Columns[1], test.ShouldEqual, "successful")

	joints := backtrack.Frames[0].Joints
	test.That(t, joints[0][2].HalfTurns, test.ShouldBeTrue)
	test.That(t, joints[0][2].Same, test.ShouldBeFalse)

	test.That(t, joints[1][2].Same, test.ShouldBeTrue)
	test.That(t, joints[1][2].HalfTurns, test.ShouldBeFalse)

	test.That(t, joints[2][2].HalfTurns, test.ShouldBeTrue)

	test.That(t, joints[3][2].HalfTurns, test.ShouldBeFalse)
	test.That(t, joints[3][2].Same, test.ShouldBeFalse)
}

func TestBuildSequenceBacktrackNotesWhenReplanFoundNothingNew(t *testing.T) {
	end := referenceframe.FrameSystemInputs{"arm": {1, 2}}
	meta := &armplanning.PlanMeta{RejectedSolutions: []armplanning.RejectedSolution{
		{Start: referenceframe.FrameSystemInputs{"arm": {0, 0}}, End: end},
	}}
	plan := motionplan.NewSimplePlan(nil, motionplan.Trajectory{{"arm": {0, 0}}, end})

	backtrack := buildSequenceBacktrack(&armplanning.PlanRequest{}, plan, meta, false)
	test.That(t, backtrack, test.ShouldNotBeNil)
	test.That(t, len(backtrack.Notes), test.ShouldEqual, 1)
}
