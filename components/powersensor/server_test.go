package powersensor_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/powersensor/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	workingPowerSensorName = "workingPS"
	failingPowerSensorName = "failingPS"
	missingPowerSensorName = "missingPS"
	errVoltageFailed       = errors.New("can't get voltage")
	errCurrentFailed       = errors.New("can't get current")
	errPowerFailed         = errors.New("can't get power")
	errPowerSensorNotFound = errors.New("not found")
	errReadingsFailed      = errors.New("can't get readings")
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

	server := powersensor.NewRPCServiceServer(powerSensorSvc).(pb.PowerSensorServiceServer)

	return server, workingPowerSensor, failingPowerSensor, nil
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

func TestServerGetReadings(t *testing.T) {
	powerSensorServer, testPowerSensor, failingPowerSensor, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	rs := map[string]interface{}{"a": 1.1, "b": 2.2}

	var extraCap map[string]interface{}
	testPowerSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		extraCap = extra
		return rs, nil
	}

	failingPowerSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return nil, errReadingsFailed
	}

	expected := map[string]*structpb.Value{}
	for k, v := range rs {
		vv, err := structpb.NewValue(v)
		test.That(t, err, test.ShouldBeNil)
		expected[k] = vv
	}
	extra, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
	test.That(t, err, test.ShouldBeNil)

	resp, err := powerSensorServer.GetReadings(context.Background(), &commonpb.GetReadingsRequest{Name: "testSensorName", Extra: extra})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Readings, test.ShouldResemble, expected)
	test.That(t, extraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

	_, err = powerSensorServer.GetReadings(context.Background(), &commonpb.GetReadingsRequest{Name: "failSensorName"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errReadingsFailed.Error())

	_, err = powerSensorServer.GetReadings(context.Background(), &commonpb.GetReadingsRequest{Name: "missingSensorName"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
}
