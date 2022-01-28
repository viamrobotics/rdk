package sensor_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
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

	injectSensor.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) { return rs, nil }

	injectSensor2.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) { return nil, errors.New("can't get readings") }

	t.Run("GetReadings", func(t *testing.T) {
		expected := make([]*structpb.Value, 0, len(rs))
		for _, r := range rs {
			v, err := structpb.NewValue(r)
			test.That(t, err, test.ShouldBeNil)
			expected = append(expected, v)
		}
		resp, err := sensorServer.GetReadings(context.Background(), &pb.SensorServiceGetReadingsRequest{Name: testSensorName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Readings, test.ShouldResemble, expected)

		_, err = sensorServer.GetReadings(context.Background(), &pb.SensorServiceGetReadingsRequest{Name: failSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get readings")

		_, err = sensorServer.GetReadings(context.Background(), &pb.SensorServiceGetReadingsRequest{Name: fakeSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a generic sensor")

		_, err = sensorServer.GetReadings(context.Background(), &pb.SensorServiceGetReadingsRequest{Name: missingSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no generic sensor")
	})
}
