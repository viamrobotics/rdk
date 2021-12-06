package gripper_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/component/gripper"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils/inject"
)

func newServer() (pb.GripperServiceServer, *inject.Gripper, *inject.Gripper, error) {
	injectGripper := &inject.Gripper{}
	injectGripper2 := &inject.Gripper{}
	grippers := map[resource.Name]interface{}{
		gripper.Named("gripper1"): injectGripper,
		gripper.Named("gripper2"): injectGripper2,
		gripper.Named("gripper3"): "notGripper",
	}
	gripperSvc, err := subtype.New(grippers)
	if err != nil {
		return nil, nil, nil, err
	}
	return gripper.NewServer(gripperSvc), injectGripper, injectGripper2, nil
}

func TestServer(t *testing.T) {
	gripperServer, injectGripper, injectGripper2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var gripperOpen string

	gripper1 := "gripper1"
	grabbed1 := true
	injectGripper.OpenFunc = func(ctx context.Context) error {
		gripperOpen = gripper1
		return nil
	}
	injectGripper.GrabFunc = func(ctx context.Context) (bool, error) { return grabbed1, nil }

	gripper2 := "gripper2"
	injectGripper2.OpenFunc = func(ctx context.Context) error {
		gripperOpen = gripper2
		return errors.New("can't open")
	}
	injectGripper2.GrabFunc = func(ctx context.Context) (bool, error) { return false, errors.New("can't grab") }

	t.Run("open", func(t *testing.T) {
		_, err := gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: "g4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

		_, err = gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: "gripper3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a gripper")

		_, err = gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: gripper1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gripperOpen, test.ShouldEqual, gripper1)

		_, err = gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: gripper2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't open")
		test.That(t, gripperOpen, test.ShouldEqual, gripper2)
	})

	t.Run("grab", func(t *testing.T) {
		_, err := gripperServer.Grab(context.Background(), &pb.GripperServiceGrabRequest{Name: "g4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

		resp, err := gripperServer.Grab(context.Background(), &pb.GripperServiceGrabRequest{Name: gripper1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Grabbed, test.ShouldEqual, grabbed1)

		resp, err = gripperServer.Grab(context.Background(), &pb.GripperServiceGrabRequest{Name: gripper2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't grab")
		test.That(t, resp, test.ShouldBeNil)
	})

}
