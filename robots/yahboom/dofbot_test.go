package yahboom

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/referenceframe"
)

func TestJointConfig(t *testing.T) {
	test.That(t, joints[0].toDegrees(joints[0].toHw(-45)), test.ShouldAlmostEqual, -45, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(0)), test.ShouldAlmostEqual, 0, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(45)), test.ShouldAlmostEqual, 45, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(90)), test.ShouldAlmostEqual, 90, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(135)), test.ShouldAlmostEqual, 135, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(200)), test.ShouldAlmostEqual, 200, .1)
}

func TestDofBotIK(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	_, mp, err := createDofBotSolver(logger)
	test.That(t, err, test.ShouldBeNil)

	goal := commonpb.Pose{X: 206.59, Y: -1.57, Z: 253.05, Theta: -180, OX: -.53, OY: 0, OZ: .85}
	_, err = mp.Plan(ctx, &goal, referenceframe.JointPosToInputs(&componentpb.ArmJointPositions{Degrees: make([]float64, 5)}))
	test.That(t, err, test.ShouldBeNil)
}
