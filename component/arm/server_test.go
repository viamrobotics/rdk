package arm_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/component/arm"
	commonpb "go.viam.com/core/proto/api/common/v1"
	componentpb "go.viam.com/core/proto/api/component/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils/inject"
)

func newServer() (pb.ArmServiceServer, *inject.Arm, *inject.Arm, error) {
	injectArm := &inject.Arm{}
	injectArm2 := &inject.Arm{}
	armSvc, err := subtype.New((map[resource.Name]interface{}{arm.Named("arm1"): injectArm, arm.Named("arm2"): injectArm2}))
	if err != nil {
		return nil, nil, nil, err
	}
	return arm.NewServer(armSvc), injectArm, injectArm2, nil
}

func TestServer(t *testing.T) {
	armServer, injectArm, injectArm2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var (
		capArmPos           *commonpb.Pose
		capArmJointPos      *componentpb.ArmJointPositions
		capArmJoint         int
		capArmJointAngleDeg float64
	)

	arm1 := "arm1"
	pos1 := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	jointPos1 := &componentpb.ArmJointPositions{Degrees: []float64{1.0, 2.0, 3.0}}
	injectArm.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos1, nil
	}
	injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*componentpb.ArmJointPositions, error) {
		return jointPos1, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose) error {
		capArmPos = ap
		return nil
	}

	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.ArmJointPositions) error {
		capArmJointPos = jp
		return nil
	}

	injectArm.JointMoveDeltaFunc = func(ctx context.Context, joint int, amountDegs float64) error {
		capArmJoint = joint
		capArmJointAngleDeg = amountDegs
		return nil
	}

	arm2 := "arm2"
	pos2 := &commonpb.Pose{X: 4, Y: 5, Z: 6}
	jointPos2 := &componentpb.ArmJointPositions{Degrees: []float64{4.0, 5.0, 6.0}}
	injectArm2.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	injectArm2.CurrentJointPositionsFunc = func(ctx context.Context) (*componentpb.ArmJointPositions, error) {
		return jointPos2, nil
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose) error {
		capArmPos = ap
		return nil
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.ArmJointPositions) error {
		capArmJointPos = jp
		return nil
	}

	injectArm2.JointMoveDeltaFunc = func(ctx context.Context, joint int, amountDegs float64) error {
		capArmJoint = joint
		capArmJointAngleDeg = amountDegs
		return nil
	}

	t.Run("arm position", func(t *testing.T) {
		resp, err := armServer.CurrentPosition(context.Background(), &pb.ArmServiceCurrentPositionRequest{Name: arm1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position.String(), test.ShouldResemble, pos1.String())

		resp, err = armServer.CurrentPosition(context.Background(), &pb.ArmServiceCurrentPositionRequest{Name: arm2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position.String(), test.ShouldResemble, pos2.String())
	})

	t.Run("move to position", func(t *testing.T) {
		_, err := armServer.MoveToPosition(context.Background(), &pb.ArmServiceMoveToPositionRequest{Name: arm1, To: pos2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos2.String())

		_, err = armServer.MoveToPosition(context.Background(), &pb.ArmServiceMoveToPositionRequest{Name: arm2, To: pos1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos1.String())
	})

	t.Run("arm joint position", func(t *testing.T) {
		resp, err := armServer.CurrentJointPositions(context.Background(), &pb.ArmServiceCurrentJointPositionsRequest{Name: arm1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions.String(), test.ShouldResemble, jointPos1.String())

		resp, err = armServer.CurrentJointPositions(context.Background(), &pb.ArmServiceCurrentJointPositionsRequest{Name: arm2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions.String(), test.ShouldResemble, jointPos2.String())
	})

	t.Run("move to joint position", func(t *testing.T) {
		_, err := armServer.MoveToJointPositions(context.Background(), &pb.ArmServiceMoveToJointPositionsRequest{Name: arm1, To: jointPos2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos2.String())

		_, err = armServer.MoveToJointPositions(context.Background(), &pb.ArmServiceMoveToJointPositionsRequest{Name: arm2, To: jointPos1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos1.String())
	})

	t.Run("joint move delta", func(t *testing.T) {
		_, err := armServer.JointMoveDelta(context.Background(), &pb.ArmServiceJointMoveDeltaRequest{Name: arm1, Joint: 10, AmountDegs: 5.5})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJoint, test.ShouldEqual, 10)
		test.That(t, capArmJointAngleDeg, test.ShouldEqual, 5.5)

		_, err = armServer.JointMoveDelta(context.Background(), &pb.ArmServiceJointMoveDeltaRequest{Name: arm2, Joint: 11, AmountDegs: 6.6})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJoint, test.ShouldEqual, 11)
		test.That(t, capArmJointAngleDeg, test.ShouldEqual, 6.6)
	})
}
