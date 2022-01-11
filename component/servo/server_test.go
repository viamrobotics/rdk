package servo_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/servo"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.ServoServiceServer, *inject.Servo, *inject.Servo, error) {
	injectServo := &inject.Servo{}
	injectServo2 := &inject.Servo{}
	resourceMap := map[resource.Name]interface{}{
		servo.Named("workingServo"): injectServo,
		servo.Named("failingServo"): injectServo2,
		servo.Named(("notAServo")):  "not a servo",
	}
	injectSvc, err := subtype.New((resourceMap))
	if err != nil {
		return nil, nil, nil, err
	}
	return servo.NewServer(injectSvc), injectServo, injectServo2, nil
}

func TestServoMove(t *testing.T) {
	servoServer, workingServo, failingServo, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	workingServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		return nil
	}
	failingServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		return errors.New("move failed")
	}

	req := pb.ServoServiceMoveRequest{Name: "workingServo"}
	resp, err := servoServer.Move(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	req = pb.ServoServiceMoveRequest{Name: "failingServo"}
	resp, err = servoServer.Move(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	req = pb.ServoServiceMoveRequest{Name: "notAServo"}
	resp, err = servoServer.Move(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestServoCurrent(t *testing.T) {
	servoServer, workingServo, failingServo, _ := newServer()

	workingServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 20, nil
	}
	failingServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 0, errors.New("current angle not readable")
	}

	req := pb.ServoServiceAngularOffsetRequest{Name: "workingServo"}
	resp, err := servoServer.AngularOffset(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	req = pb.ServoServiceAngularOffsetRequest{Name: "failingServo"}
	resp, err = servoServer.AngularOffset(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	req = pb.ServoServiceAngularOffsetRequest{Name: "notAServo"}
	resp, err = servoServer.AngularOffset(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
}
