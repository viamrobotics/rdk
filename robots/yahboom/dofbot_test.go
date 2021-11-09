package yahboom

import (
	"context"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/test"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
)

func TestJointConfig(t *testing.T) {
	test.That(t, joints[0].toDegrees(joints[0].toHw(0)), test.ShouldAlmostEqual, 0)
	test.That(t, joints[0].toDegrees(joints[0].toHw(45)), test.ShouldAlmostEqual, 45)
	test.That(t, joints[0].toDegrees(joints[0].toHw(90)), test.ShouldAlmostEqual, 90)
	test.That(t, joints[0].toDegrees(joints[0].toHw(135)), test.ShouldAlmostEqual, 135)
	test.That(t, joints[0].toDegrees(joints[0].toHw(200)), test.ShouldAlmostEqual, 200, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(300)), test.ShouldAlmostEqual, 300, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(350)), test.ShouldAlmostEqual, 350, .1)
}

func TestDofBotIK(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	model, err := dofbotModel()
	test.That(t, err, test.ShouldBeNil)

	ik, err := kinematics.CreateCombinedIKSolver(model, logger, 4)
	test.That(t, err, test.ShouldBeNil)

	goal := pb.ArmPosition{X: 206.59, Y: -1.57, Z: 253.05, Theta: -180, OX: -.53, OY: 0, OZ: .85}
	_, err = ik.Solve(ctx, &goal, referenceframe.JointPosToInputs(&pb.JointPositions{Degrees: make([]float64, 5)}))
	test.That(t, err, test.ShouldBeNil)
}
