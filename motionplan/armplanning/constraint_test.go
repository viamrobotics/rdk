package armplanning

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIKTolerances(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	fs.AddFrame(m, fs.World())

	goal := referenceframe.FrameSystemPoses{m.Name(): referenceframe.NewPoseInFrame(
		referenceframe.World,
		spatial.NewPoseFromProtobuf(&commonpb.Pose{X: -46, Y: 0, Z: 372, OX: -1.78, OY: -3.3, OZ: -1.11}),
	)}

	seed := referenceframe.NewLinearInputs()
	seed.Put(m.Name(), make([]referenceframe.Input, 6))

	// Create PlanRequest to use the new API
	request := &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{NewPlanState(goal, nil)},
		StartState:     NewPlanState(nil, seed.ToFrameSystemInputs()),
		PlannerOptions: NewBasicPlannerOptions(),
		Constraints:    &motionplan.Constraints{},
	}

	pc, err := newPlanContext(ctx, logger, request, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, seed, goal)
	test.That(t, err, test.ShouldBeNil)

	mp, err := newCBiRRTMotionPlanner(ctx, pc, psc, logger.Sublogger("cbirrt"))
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	_, err = mp.planForTest(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	opt := NewBasicPlannerOptions()
	opt.GoalMetricType = motionplan.PositionOnly
	opt.SetMaxSolutions(50)

	request2 := &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{NewPlanState(goal, nil)},
		StartState:     NewPlanState(nil, seed.ToFrameSystemInputs()),
		PlannerOptions: opt,
		Constraints:    &motionplan.Constraints{},
	}

	pc2, err := newPlanContext(ctx, logger, request2, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc2, err := newPlanSegmentContext(ctx, pc2, seed, goal)
	test.That(t, err, test.ShouldBeNil)

	mp2, err := newCBiRRTMotionPlanner(ctx, pc2, psc2, logger.Sublogger("cbirrt"))
	test.That(t, err, test.ShouldBeNil)
	_, err = mp2.planForTest(ctx)
	test.That(t, err, test.ShouldBeNil)
}
