package gripper_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/gripper"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.GripperServiceServer, *inject.Gripper, *inject.Gripper, error) {
	injectGripper := &inject.Gripper{}
	injectGripper2 := &inject.Gripper{}
	grippers := map[resource.Name]interface{}{
		gripper.Named(testGripperName):  injectGripper,
		gripper.Named(testGripperName2): injectGripper2,
		gripper.Named(fakeGripperName):  "notGripper",
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

	success1 := true
	injectGripper.OpenFunc = func(ctx context.Context) error {
		gripperOpen = testGripperName
		return nil
	}
	injectGripper.GrabFunc = func(ctx context.Context) (bool, error) { return success1, nil }

	injectGripper2.OpenFunc = func(ctx context.Context) error {
		gripperOpen = testGripperName2
		return errors.New("can't open")
	}
	injectGripper2.GrabFunc = func(ctx context.Context) (bool, error) { return false, errors.New("can't grab") }

	t.Run("open", func(t *testing.T) {
		_, err := gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: missingGripperName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

		_, err = gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: fakeGripperName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a gripper")

		_, err = gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: testGripperName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gripperOpen, test.ShouldEqual, testGripperName)

		_, err = gripperServer.Open(context.Background(), &pb.GripperServiceOpenRequest{Name: testGripperName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't open")
		test.That(t, gripperOpen, test.ShouldEqual, testGripperName2)
	})

	t.Run("grab", func(t *testing.T) {
		_, err := gripperServer.Grab(context.Background(), &pb.GripperServiceGrabRequest{Name: missingGripperName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

		resp, err := gripperServer.Grab(context.Background(), &pb.GripperServiceGrabRequest{Name: testGripperName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Success, test.ShouldEqual, success1)

		resp, err = gripperServer.Grab(context.Background(), &pb.GripperServiceGrabRequest{Name: testGripperName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't grab")
		test.That(t, resp, test.ShouldBeNil)
	})
}
