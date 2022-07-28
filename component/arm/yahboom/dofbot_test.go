package yahboom

import (
	"context"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/arm/v1"
)

func TestJointConfig(t *testing.T) {
	test.That(t, joints[0].toValues(joints[0].toHw(-45)), test.ShouldAlmostEqual, -45, .1)
	test.That(t, joints[0].toValues(joints[0].toHw(0)), test.ShouldAlmostEqual, 0, .1)
	test.That(t, joints[0].toValues(joints[0].toHw(45)), test.ShouldAlmostEqual, 45, .1)
	test.That(t, joints[0].toValues(joints[0].toHw(90)), test.ShouldAlmostEqual, 90, .1)
	test.That(t, joints[0].toValues(joints[0].toHw(135)), test.ShouldAlmostEqual, 135, .1)
	test.That(t, joints[0].toValues(joints[0].toHw(200)), test.ShouldAlmostEqual, 200, .1)
}

func TestDofBotIK(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	model, err := dofbotModel()
	test.That(t, err, test.ShouldBeNil)
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, 4, rand.New(rand.NewSource(1)), logger)
	test.That(t, err, test.ShouldBeNil)

	goal := commonpb.Pose{X: 206.59, Y: -1.57, Z: 253.05, Theta: -180, OX: -.53, OY: 0, OZ: .85}
	opt := motionplan.NewDefaultPlannerOptions()
	opt.SetMetric(motionplan.NewPositionOnlyMetric())
	_, err = mp.Plan(ctx, &goal, model.InputFromProtobuf(&componentpb.JointPositions{Values: make([]float64, 5)}), opt)
	test.That(t, err, test.ShouldBeNil)
}
