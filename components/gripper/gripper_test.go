package gripper_test

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testGripperName    = "gripper1"
	testGripperName2   = "gripper2"
	failGripperName    = "gripper3"
	fakeGripperName    = "gripper4"
	missingGripperName = "gripper5"
)

func TestCreateStatus(t *testing.T) {
	_, err := gripper.CreateStatus(context.Background(), testutils.NewUnimplementedResource(generic.Named("foo")))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation")

	t.Run("is moving", func(t *testing.T) {
		status := &commonpb.ActuatorStatus{
			IsMoving: true,
		}

		injectGripper := &inject.Gripper{}
		injectGripper.IsMovingFunc = func(context.Context) (bool, error) {
			return true, nil
		}
		status1, err := gripper.CreateStatus(context.Background(), injectGripper)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype, ok := registry.ResourceSubtypeLookup(gripper.Subtype)
		test.That(t, ok, test.ShouldBeTrue)
		status2, err := resourceSubtype.Status(context.Background(), injectGripper)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("is not moving", func(t *testing.T) {
		status := &commonpb.ActuatorStatus{
			IsMoving: false,
		}

		injectGripper := &inject.Gripper{}
		injectGripper.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}
		status1, err := gripper.CreateStatus(context.Background(), injectGripper)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)
	})
}
