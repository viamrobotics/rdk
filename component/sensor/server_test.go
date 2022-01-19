package sensor_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/sensor"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testSensorName    = "sensor1"
	failSensorName    = "sensor2"
	fakeSensorName    = "sensor3"
	missingSensorName = "sensor4"
)

func newServer() (pb.SensorServiceServer, *inject.Sensor, *inject.Sensor, error) {
	injectSensor := &inject.Sensor{}
	injectSensor2 := &inject.Sensor{}
	sensors := map[resource.Name]interface{}{
		sensor.Named(testSensorName): injectSensor,
		sensor.Named(failSensorName): injectSensor2,
		sensor.Named(fakeSensorName): "notSensor",
	}
	sensorSvc, err := subtype.New(sensors)
	if err != nil {
		return nil, nil, nil, err
	}
	return sensor.NewServer(sensorSvc), injectSensor, injectSensor2, nil
}

func TestServer(t *testing.T) {
	sensorServer, injectSensor, injectSensor2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	rs := []interface{}{1.1, 2.2}
	desc := sensor.Description{sensor.Type("sensor"), ""}

	injectSensor.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) { return rs, nil }
	injectSensor.DescFunc = func(context.Context) (sensor.Description, error) { return desc, nil }

	injectSensor2.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) { return nil, errors.New("can't get readings") }
	injectSensor2.DescFunc = func(context.Context) (sensor.Description, error) {
		return sensor.Description{}, errors.New("can't get desc")
	}

	t.Run("Readings", func(t *testing.T) {
		expected := make([]*structpb.Value, 0, len(rs))
		for _, r := range rs {
			v, err := structpb.NewValue(r)
			test.That(t, err, test.ShouldBeNil)
			expected = append(expected, v)
		}
		resp, err := sensorServer.Readings(context.Background(), &pb.SensorServiceReadingsRequest{Name: testSensorName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Readings, test.ShouldResemble, expected)

		_, err = sensorServer.Readings(context.Background(), &pb.SensorServiceReadingsRequest{Name: failSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get readings")

		_, err = sensorServer.Readings(context.Background(), &pb.SensorServiceReadingsRequest{Name: fakeSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a Sensor")

		_, err = sensorServer.Readings(context.Background(), &pb.SensorServiceReadingsRequest{Name: missingSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no Sensor")
	})

	t.Run("Desc", func(t *testing.T) {
		resp, err := sensorServer.Desc(context.Background(), &pb.SensorServiceDescRequest{Name: testSensorName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Desc, test.ShouldResemble, &commonpb.SensorDescription{Type: string(desc.Type), Path: desc.Path})

		_, err = sensorServer.Desc(context.Background(), &pb.SensorServiceDescRequest{Name: failSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get desc")

		_, err = sensorServer.Desc(context.Background(), &pb.SensorServiceDescRequest{Name: fakeSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a Sensor")

		_, err = sensorServer.Desc(context.Background(), &pb.SensorServiceDescRequest{Name: missingSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no Sensor")
	})
}
