package armplanning

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIKTolerances(t *testing.T) {
	logger := logging.NewTestLogger(t)

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/ur5eDH.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("")
	fs.AddFrame(m, fs.World())

	goal := frame.FrameSystemPoses{m.Name(): frame.NewPoseInFrame(
		frame.World,
		spatial.NewPoseFromProtobuf(&commonpb.Pose{X: -46, Y: 0, Z: 372, OX: -1.78, OY: -3.3, OZ: -1.11}),
	)}

	seed := frame.FrameSystemInputs{m.Name(): frame.FloatsToInputs(make([]float64, 6))}

	// Create PlanRequest to use the new API
	request := &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{NewPlanState(goal, nil)},
		StartState: NewPlanState(nil, seed),
		PlannerOptions: NewBasicPlannerOptions(),
		Constraints: &motionplan.Constraints{},
	}

	pc, err := newPlanContext(logger, request)
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(pc, seed, goal)
	test.That(t, err, test.ShouldBeNil)

	mp, err := newCBiRRTMotionPlanner(pc, psc)
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	_, err = mp.planForTest(context.Background())
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	opt := NewBasicPlannerOptions()
	opt.GoalMetricType = motionplan.PositionOnly
	opt.SetMaxSolutions(50)

	request2 := &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{NewPlanState(goal, nil)},
		StartState: NewPlanState(nil, seed),
		PlannerOptions: opt,
		Constraints: &motionplan.Constraints{},
	}

	pc2, err := newPlanContext(logger, request2)
	test.That(t, err, test.ShouldBeNil)

	psc2, err := newPlanSegmentContext(pc2, seed, goal)
	test.That(t, err, test.ShouldBeNil)

	mp2, err := newCBiRRTMotionPlanner(pc2, psc2)
	test.That(t, err, test.ShouldBeNil)
	_, err = mp2.planForTest(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
