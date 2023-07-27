package servo_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/servo/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errMoveFailed         = errors.New("move failed")
	errPositionUnreadable = errors.New("current angle not readable")
	errStopFailed         = errors.New("stop failed")
)

func newServer() (pb.ServoServiceServer, *inject.Servo, *inject.Servo, error) {
	injectServo := &inject.Servo{}
	injectServo2 := &inject.Servo{}
	resourceMap := map[resource.Name]servo.Servo{
		servo.Named(testServoName): injectServo,
		servo.Named(failServoName): injectServo2,
	}
	injectSvc, err := resource.NewAPIResourceCollection(servo.API, resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return servo.NewRPCServiceServer(injectSvc).(pb.ServoServiceServer), injectServo, injectServo2, nil
}

func TestServoMove(t *testing.T) {
	servoServer, workingServo, failingServo, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var actualExtra map[string]interface{}

	workingServo.MoveFunc = func(ctx context.Context, angle uint32, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	failingServo.MoveFunc = func(ctx context.Context, angle uint32, extra map[string]interface{}) error {
		return errMoveFailed
	}

	extra := map[string]interface{}{"foo": "Move"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)

	req := pb.MoveRequest{Name: testServoName, Extra: ext}
	resp, err := servoServer.Move(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualExtra, test.ShouldResemble, extra)

	req = pb.MoveRequest{Name: failServoName}
	resp, err = servoServer.Move(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errMoveFailed.Error())

	req = pb.MoveRequest{Name: fakeServoName}
	resp, err = servoServer.Move(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestServoGetPosition(t *testing.T) {
	servoServer, workingServo, failingServo, _ := newServer()

	var actualExtra map[string]interface{}

	workingServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		actualExtra = extra
		return 20, nil
	}
	failingServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 0, errPositionUnreadable
	}

	extra := map[string]interface{}{"foo": "Move"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)

	req := pb.GetPositionRequest{Name: testServoName, Extra: ext}
	resp, err := servoServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualExtra, test.ShouldResemble, extra)

	req = pb.GetPositionRequest{Name: failServoName}
	resp, err = servoServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPositionUnreadable.Error())

	req = pb.GetPositionRequest{Name: fakeServoName}
	resp, err = servoServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestServoStop(t *testing.T) {
	servoServer, workingServo, failingServo, _ := newServer()

	var actualExtra map[string]interface{}

	workingServo.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	failingServo.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopFailed
	}

	extra := map[string]interface{}{"foo": "Move"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)

	req := pb.StopRequest{Name: testServoName, Extra: ext}
	_, err = servoServer.Stop(context.Background(), &req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualExtra, test.ShouldResemble, extra)

	req = pb.StopRequest{Name: failServoName}
	_, err = servoServer.Stop(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errStopFailed.Error())

	req = pb.StopRequest{Name: fakeServoName}
	_, err = servoServer.Stop(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
}
