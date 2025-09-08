package baseplanning

import (
	"context"
	"math/rand"
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
	mp, err := newCBiRRTMotionPlanner(
		fs, rand.New(rand.NewSource(1)), logger, NewBasicPlannerOptions(), motionplan.NewEmptyConstraintHandler(), nil)
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	goal := &PlanState{poses: frame.FrameSystemPoses{m.Name(): frame.NewPoseInFrame(
		frame.World,
		spatial.NewPoseFromProtobuf(&commonpb.Pose{X: -46, Y: 0, Z: 372, OX: -1.78, OY: -3.3, OZ: -1.11}),
	)}}
	seed := &PlanState{configuration: map[string][]frame.Input{m.Name(): frame.FloatsToInputs(make([]float64, 6))}}
	_, err = mp.plan(context.Background(), seed, goal)
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	opt := NewBasicPlannerOptions()
	opt.GoalMetricType = motionplan.PositionOnly
	opt.SetMaxSolutions(50)
	mp, err = newCBiRRTMotionPlanner(fs, rand.New(rand.NewSource(1)), logger, opt, motionplan.NewEmptyConstraintHandler(), nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = mp.plan(context.Background(), seed, goal)
	test.That(t, err, test.ShouldBeNil)
}
