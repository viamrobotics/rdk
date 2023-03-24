package servo_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/servo/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestCreateStatus(t *testing.T) {
	_, err := servo.CreateStatus(context.Background(), testutils.NewUnimplementedResource(generic.Named("foo")))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation")

	status := &pb.Status{PositionDeg: uint32(8), IsMoving: true}

	injectServo := &inject.Servo{}
	injectServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return status.PositionDeg, nil
	}
	injectServo.IsMovingFunc = func(context.Context) (bool, error) {
		return true, nil
	}

	t.Run("working", func(t *testing.T) {
		status1, err := servo.CreateStatus(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype, ok := registry.ResourceSubtypeLookup(servo.Subtype)
		test.That(t, ok, test.ShouldBeTrue)
		status2, err := resourceSubtype.Status(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("not moving", func(t *testing.T) {
		injectServo.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		status2 := &pb.Status{PositionDeg: uint32(8), IsMoving: false}
		status1, err := servo.CreateStatus(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status2)
	})

	t.Run("fail on Position", func(t *testing.T) {
		errFail := errors.New("can't get position")
		injectServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
			return 0, errFail
		}
		_, err = servo.CreateStatus(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}
