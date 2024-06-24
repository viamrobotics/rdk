package motor_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errPositionUnavailable = errors.New("position unavailable")
	errResetZeroFailed     = errors.New("set to zero failed")
	errPropertiesNotFound  = errors.New("properties not found")
	errGetPropertiesFailed = errors.New("get properties failed")
	errSetPowerFailed      = errors.New("set power failed")
	errGoForFailed         = errors.New("go for failed")
	errStopFailed          = errors.New("stop failed")
	errIsPoweredFailed     = errors.New("could not determine if motor is on")
	errGoToFailed          = errors.New("go to failed")
	errSetRPMFailed        = errors.New("set rpm failed")
)

func newServer() (pb.MotorServiceServer, *inject.Motor, *inject.Motor, error) {
	injectMotor1 := &inject.Motor{}
	injectMotor2 := &inject.Motor{}

	resourceMap := map[resource.Name]motor.Motor{
		motor.Named(testMotorName): injectMotor1,
		motor.Named(failMotorName): injectMotor2,
	}

	injectSvc, err := resource.NewAPIResourceCollection(motor.API, resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return motor.NewRPCServiceServer(injectSvc).(pb.MotorServiceServer), injectMotor1, injectMotor2, nil
}

//nolint:dupl
func TestServerSetPower(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.SetPowerRequest{Name: fakeMotorName}
	resp, err := motorServer.SetPower(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		return errSetPowerFailed
	}
	req = pb.SetPowerRequest{Name: failMotorName, PowerPct: 0.5}
	resp, err = motorServer.SetPower(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		return nil
	}
	req = pb.SetPowerRequest{Name: testMotorName, PowerPct: 0.5}
	resp, err = motorServer.SetPower(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerGoFor(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.GoForRequest{Name: fakeMotorName}
	resp, err := motorServer.GoFor(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return errGoForFailed
	}
	req = pb.GoForRequest{Name: failMotorName, Rpm: 42.0, Revolutions: 42.1}
	resp, err = motorServer.GoFor(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return nil
	}
	req = pb.GoForRequest{Name: testMotorName, Rpm: 42.0, Revolutions: 42.1}
	resp, err = motorServer.GoFor(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerPosition(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.GetPositionRequest{Name: fakeMotorName}
	resp, err := motorServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)

	failingMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, errPositionUnavailable
	}
	req = pb.GetPositionRequest{Name: failMotorName}
	resp, err = motorServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 42.0, nil
	}
	req = pb.GetPositionRequest{Name: testMotorName}
	resp, err = motorServer.GetPosition(context.Background(), &req)
	test.That(t, resp.GetPosition(), test.ShouldEqual, 42.0)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerGetProperties(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.GetPropertiesRequest{Name: fakeMotorName}
	resp, err := motorServer.GetProperties(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
		return motor.Properties{}, errGetPropertiesFailed
	}
	req = pb.GetPropertiesRequest{Name: failMotorName}
	resp, err = motorServer.GetProperties(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
		return motor.Properties{
			PositionReporting: true,
		}, nil
	}
	req = pb.GetPropertiesRequest{Name: testMotorName}
	resp, err = motorServer.GetProperties(context.Background(), &req)
	test.That(t, resp.GetPositionReporting(), test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerStop(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.StopRequest{Name: fakeMotorName}
	resp, err := motorServer.Stop(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopFailed
	}
	req = pb.StopRequest{Name: failMotorName}
	resp, err = motorServer.Stop(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}
	req = pb.StopRequest{Name: testMotorName}
	resp, err = motorServer.Stop(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerIsOn(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.IsPoweredRequest{Name: fakeMotorName}
	resp, err := motorServer.IsPowered(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return false, 0.0, errIsPoweredFailed
	}
	req = pb.IsPoweredRequest{Name: failMotorName}
	resp, err = motorServer.IsPowered(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return true, 1.0, nil
	}
	req = pb.IsPoweredRequest{Name: testMotorName}
	resp, err = motorServer.IsPowered(context.Background(), &req)
	test.That(t, resp.GetIsOn(), test.ShouldBeTrue)
	test.That(t, resp.GetPowerPct(), test.ShouldEqual, 1.0)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerGoTo(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.GoToRequest{Name: fakeMotorName}
	resp, err := motorServer.GoTo(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		return errGoToFailed
	}
	req = pb.GoToRequest{Name: failMotorName, Rpm: 20.0, PositionRevolutions: 2.5}
	resp, err = motorServer.GoTo(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		return nil
	}
	req = pb.GoToRequest{Name: testMotorName, Rpm: 20.0, PositionRevolutions: 2.5}
	resp, err = motorServer.GoTo(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerSetRPM(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.SetRPMRequest{Name: fakeMotorName}
	resp, err := motorServer.SetRPM(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.SetRPMFunc = func(ctx context.Context, rpm float64, extra map[string]interface{}) error {
		return errSetRPMFailed
	}
	req = pb.SetRPMRequest{Name: failMotorName, Rpm: 20.0}
	resp, err = motorServer.SetRPM(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.SetRPMFunc = func(ctx context.Context, rpm float64, extra map[string]interface{}) error {
		return nil
	}
	req = pb.SetRPMRequest{Name: testMotorName, Rpm: 20.0}
	resp, err = motorServer.SetRPM(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerResetZeroPosition(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.ResetZeroPositionRequest{Name: fakeMotorName}
	resp, err := motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		return errResetZeroFailed
	}
	req = pb.ResetZeroPositionRequest{Name: failMotorName, Offset: 1.1}
	resp, err = motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		return nil
	}
	req = pb.ResetZeroPositionRequest{Name: testMotorName, Offset: 1.1}
	resp, err = motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerExtraParams(t *testing.T) {
	motorServer, workingMotor, _, _ := newServer()

	var actualExtra map[string]interface{}
	workingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}

	ext, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	req := pb.ResetZeroPositionRequest{Name: testMotorName, Offset: 1.1, Extra: ext}
	resp, err := motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualExtra["foo"], test.ShouldEqual, expectedExtra["foo"])
	test.That(t, actualExtra["baz"], test.ShouldResemble, expectedExtra["baz"])
}
