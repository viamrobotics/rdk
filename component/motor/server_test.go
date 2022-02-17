package motor_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/motor"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.MotorServiceServer, *inject.Motor, *inject.Motor, error) {
	injectMotor1 := &inject.Motor{}
	injectMotor2 := &inject.Motor{}

	resourceMap := map[resource.Name]interface{}{
		motor.Named(testMotorName): injectMotor1,
		motor.Named(failMotorName): injectMotor2,
		motor.Named(fakeMotorName): "not a motor",
	}

	injectSvc, err := subtype.New(resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return motor.NewServer(injectSvc), injectMotor1, injectMotor2, nil
}

//nolint:dupl
func TestServerSetPower(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceSetPowerRequest{Name: fakeMotorName}
	resp, err := motorServer.SetPower(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
		return errors.New("set power failed")
	}
	req = pb.MotorServiceSetPowerRequest{Name: failMotorName, PowerPct: 0.5}
	resp, err = motorServer.SetPower(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}
	req = pb.MotorServiceSetPowerRequest{Name: testMotorName, PowerPct: 0.5}
	resp, err = motorServer.SetPower(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerGoFor(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceGoForRequest{Name: fakeMotorName}
	resp, err := motorServer.GoFor(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64) error {
		return errors.New("go for failed")
	}
	req = pb.MotorServiceGoForRequest{Name: failMotorName, Rpm: 42.0, Revolutions: 42.1}
	resp, err = motorServer.GoFor(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64) error {
		return nil
	}
	req = pb.MotorServiceGoForRequest{Name: testMotorName, Rpm: 42.0, Revolutions: 42.1}
	resp, err = motorServer.GoFor(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerPosition(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceGetPositionRequest{Name: fakeMotorName}
	resp, err := motorServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
		return 0, errors.New("position unavailable")
	}
	req = pb.MotorServiceGetPositionRequest{Name: failMotorName}
	resp, err = motorServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
		return 42.0, nil
	}
	req = pb.MotorServiceGetPositionRequest{Name: testMotorName}
	resp, err = motorServer.GetPosition(context.Background(), &req)
	test.That(t, resp.GetPosition(), test.ShouldEqual, 42.0)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerGetFeatures(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceGetFeaturesRequest{Name: fakeMotorName}
	resp, err := motorServer.GetFeatures(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return nil, errors.New("unable to get supported features")
	}
	req = pb.MotorServiceGetFeaturesRequest{Name: failMotorName}
	resp, err = motorServer.GetFeatures(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{
			motor.PositionReporting: true,
		}, nil
	}
	req = pb.MotorServiceGetFeaturesRequest{Name: testMotorName}
	resp, err = motorServer.GetFeatures(context.Background(), &req)
	test.That(t, resp.GetPositionReporting(), test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerStop(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceStopRequest{Name: fakeMotorName}
	resp, err := motorServer.Stop(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.StopFunc = func(ctx context.Context) error {
		return errors.New("stop failed")
	}
	req = pb.MotorServiceStopRequest{Name: failMotorName}
	resp, err = motorServer.Stop(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.StopFunc = func(ctx context.Context) error {
		return nil
	}
	req = pb.MotorServiceStopRequest{Name: testMotorName}
	resp, err = motorServer.Stop(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerIsOn(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceIsPoweredRequest{Name: fakeMotorName}
	resp, err := motorServer.IsPowered(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
		return false, errors.New("could not determine if motor is on")
	}
	req = pb.MotorServiceIsPoweredRequest{Name: failMotorName}
	resp, err = motorServer.IsPowered(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}
	req = pb.MotorServiceIsPoweredRequest{Name: testMotorName}
	resp, err = motorServer.IsPowered(context.Background(), &req)
	test.That(t, resp.GetIsOn(), test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerGoTo(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceGoToRequest{Name: fakeMotorName}
	resp, err := motorServer.GoTo(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.GoToFunc = func(ctx context.Context, rpm, position float64) error {
		return errors.New("go to failed")
	}
	req = pb.MotorServiceGoToRequest{Name: failMotorName, Rpm: 20.0, PositionRevolutions: 2.5}
	resp, err = motorServer.GoTo(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.GoToFunc = func(ctx context.Context, rpm, position float64) error {
		return nil
	}
	req = pb.MotorServiceGoToRequest{Name: testMotorName, Rpm: 20.0, PositionRevolutions: 2.5}
	resp, err = motorServer.GoTo(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestServerResetZeroPosition(t *testing.T) {
	motorServer, workingMotor, failingMotor, _ := newServer()

	// fails on a bad motor
	req := pb.MotorServiceResetZeroPositionRequest{Name: fakeMotorName}
	resp, err := motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error {
		return errors.New("set to zero failed")
	}
	req = pb.MotorServiceResetZeroPositionRequest{Name: failMotorName, Offset: 1.1}
	resp, err = motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error {
		return nil
	}
	req = pb.MotorServiceResetZeroPositionRequest{Name: testMotorName, Offset: 1.1}
	resp, err = motorServer.ResetZeroPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}
