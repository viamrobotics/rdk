package arm_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.ArmServiceServer, *inject.Arm, *inject.Arm, error) {
	injectArm := &inject.Arm{}
	injectArm2 := &inject.Arm{}
	arms := map[resource.Name]interface{}{
		arm.Named("arm1"): injectArm,
		arm.Named("arm2"): injectArm2,
		arm.Named("arm3"): "notArm",
	}
	armSvc, err := subtype.New(arms)
	if err != nil {
		return nil, nil, nil, err
	}
	return arm.NewServer(armSvc), injectArm, injectArm2, nil
}

func TestServer(t *testing.T) {
	armServer, injectArm, injectArm2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var (
		capArmPos      *commonpb.Pose
		capArmJointPos *pb.ArmJointPositions
	)

	arm1 := "arm1"
	pos1 := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	jointPos1 := &pb.ArmJointPositions{Degrees: []float64{1.0, 2.0, 3.0}}
	injectArm.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos1, nil
	}
	injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.ArmJointPositions, error) {
		return jointPos1, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose) error {
		capArmPos = ap
		return nil
	}

	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.ArmJointPositions) error {
		capArmJointPos = jp
		return nil
	}

	arm2 := "arm2"
	pos2 := &commonpb.Pose{X: 4, Y: 5, Z: 6}
	jointPos2 := &pb.ArmJointPositions{Degrees: []float64{4.0, 5.0, 6.0}}
	injectArm2.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	injectArm2.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.ArmJointPositions, error) {
		return jointPos2, nil
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose) error {
		capArmPos = ap
		return nil
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.ArmJointPositions) error {
		capArmJointPos = jp
		return nil
	}

	t.Run("arm position", func(t *testing.T) {
		_, err := armServer.CurrentPosition(context.Background(), &pb.ArmServiceCurrentPositionRequest{Name: "a4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

		_, err = armServer.CurrentPosition(context.Background(), &pb.ArmServiceCurrentPositionRequest{Name: "arm3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an arm")

		resp, err := armServer.CurrentPosition(context.Background(), &pb.ArmServiceCurrentPositionRequest{Name: arm1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position.String(), test.ShouldResemble, pos1.String())

		resp, err = armServer.CurrentPosition(context.Background(), &pb.ArmServiceCurrentPositionRequest{Name: arm2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position.String(), test.ShouldResemble, pos2.String())
	})

	//nolint:dupl
	t.Run("move to position", func(t *testing.T) {
		_, err = armServer.MoveToPosition(context.Background(), &pb.ArmServiceMoveToPositionRequest{Name: "a4", To: pos2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

		_, err := armServer.MoveToPosition(context.Background(), &pb.ArmServiceMoveToPositionRequest{Name: arm1, To: pos2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos2.String())

		_, err = armServer.MoveToPosition(context.Background(), &pb.ArmServiceMoveToPositionRequest{Name: arm2, To: pos1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos1.String())
	})

	t.Run("arm joint position", func(t *testing.T) {
		_, err := armServer.CurrentJointPositions(context.Background(), &pb.ArmServiceCurrentJointPositionsRequest{Name: "a4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

		resp, err := armServer.CurrentJointPositions(context.Background(), &pb.ArmServiceCurrentJointPositionsRequest{Name: arm1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions.String(), test.ShouldResemble, jointPos1.String())

		resp, err = armServer.CurrentJointPositions(context.Background(), &pb.ArmServiceCurrentJointPositionsRequest{Name: arm2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions.String(), test.ShouldResemble, jointPos2.String())
	})

	//nolint:dupl
	t.Run("move to joint position", func(t *testing.T) {
		_, err = armServer.MoveToJointPositions(context.Background(), &pb.ArmServiceMoveToJointPositionsRequest{Name: "a4", To: jointPos2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

		_, err := armServer.MoveToJointPositions(context.Background(), &pb.ArmServiceMoveToJointPositionsRequest{Name: arm1, To: jointPos2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos2.String())

		_, err = armServer.MoveToJointPositions(context.Background(), &pb.ArmServiceMoveToJointPositionsRequest{Name: arm2, To: jointPos1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos1.String())
	})
}
