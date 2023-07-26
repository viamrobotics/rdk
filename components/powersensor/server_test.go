package powersensor_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/component/powersensor/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	workingPowerSensorName = "ps1"
	failingPowerSensorName = "ps2"
	missingPowerSensorName = "ps3"
	errVoltageFailed       = errors.New("can't get voltage")
	errCurrentFailed       = errors.New("can't get current")
	errPowerFailed         = errors.New("can't get power")
	errPowerSensorNotFound = errors.New("not found")
)

func newServer() (pb.PowerSensorServiceServer, *inject.PowerSensor, *inject.PowerSensor, error) {
	workingPowerSensor := &inject.PowerSensor{}
	failingPowerSensor := &inject.PowerSensor{}
	powerSensors := map[resource.Name]powersensor.PowerSensor{
		powersensor.Named(workingPowerSensorName): workingPowerSensor,
		powersensor.Named(failingPowerSensorName): failingPowerSensor,
	}

	powerSensorSvc, err := resource.NewAPIResourceCollection(sensor.API, powerSensors)
	if err != nil {
		return nil, nil, nil, err
	}

	return powersensor.NewRPCServiceServer(powerSensorSvc).(pb.PowerSensorServiceServer), workingPowerSensor, failingPowerSensor, nil
}

//nolint:dupl
func TestServerGetVoltage(t *testing.T) {
	powerSensorServer, testPowerSensor, failingPowerSensor, err := newServer()
	test.That(t, err, test.ShouldBeNil)
	volts := 4.8
	isAC := false

	// successful
	testPowerSensor.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return volts, isAC, nil
	}
	req := &pb.GetVoltageRequest{Name: workingPowerSensorName}
	resp, err := powerSensorServer.GetVoltage(context.Background(), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Volts, test.ShouldEqual, volts)
	test.That(t, resp.IsAc, test.ShouldEqual, isAC)

	// fails on bad power sensor
	failingPowerSensor.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 0, false, errVoltageFailed
	}
	req = &pb.GetVoltageRequest{Name: failingPowerSensorName}
	resp, err = powerSensorServer.GetVoltage(context.Background(), req)
	test.That(t, err, test.ShouldBeError, errVoltageFailed)
	test.That(t, resp, test.ShouldBeNil)

	// missing power sensor
	req = &pb.GetVoltageRequest{Name: missingPowerSensorName}
	resp, err = powerSensorServer.GetVoltage(context.Background(), req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPowerSensorNotFound.Error())
	test.That(t, resp, test.ShouldBeNil)
}

//nolint:dupl
func TestServerGetCurrent(t *testing.T) {
	powerSensorServer, testPowerSensor, failingPowerSensor, err := newServer()
	test.That(t, err, test.ShouldBeNil)
	amps := 4.8
	isAC := false

	// successful
	testPowerSensor.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return amps, isAC, nil
	}
	req := &pb.GetCurrentRequest{Name: workingPowerSensorName}
	resp, err := powerSensorServer.GetCurrent(context.Background(), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Amperes, test.ShouldEqual, amps)
	test.That(t, resp.IsAc, test.ShouldEqual, isAC)

	// fails on bad power sensor
	failingPowerSensor.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 0, false, errCurrentFailed
	}
	req = &pb.GetCurrentRequest{Name: failingPowerSensorName}
	resp, err = powerSensorServer.GetCurrent(context.Background(), req)
	test.That(t, err, test.ShouldBeError, errCurrentFailed)
	test.That(t, resp, test.ShouldBeNil)

	// missing power sensor
	req = &pb.GetCurrentRequest{Name: missingPowerSensorName}
	resp, err = powerSensorServer.GetCurrent(context.Background(), req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPowerSensorNotFound.Error())
	test.That(t, resp, test.ShouldBeNil)
}

func TestServerGetPower(t *testing.T) {
	powerSensorServer, testPowerSensor, failingPowerSensor, err := newServer()
	test.That(t, err, test.ShouldBeNil)
	watts := 4.8

	// successful
	testPowerSensor.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return watts, nil
	}
	req := &pb.GetPowerRequest{Name: workingPowerSensorName}
	resp, err := powerSensorServer.GetPower(context.Background(), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Watts, test.ShouldEqual, watts)

	// fails on bad power sensor
	failingPowerSensor.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, errPowerFailed
	}
	req = &pb.GetPowerRequest{Name: failingPowerSensorName}
	resp, err = powerSensorServer.GetPower(context.Background(), req)
	test.That(t, err, test.ShouldBeError, errPowerFailed)
	test.That(t, resp, test.ShouldBeNil)

	// missing power sensor
	req = &pb.GetPowerRequest{Name: missingPowerSensorName}
	resp, err = powerSensorServer.GetPower(context.Background(), req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPowerSensorNotFound.Error())
	test.That(t, resp, test.ShouldBeNil)
}
