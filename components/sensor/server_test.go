package sensor_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/component/sensor/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/sensor"
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

	rs := map[string]interface{}{"a": 1.1, "b": 2.2}

	var extraCap map[string]interface{}
	injectSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		extraCap = extra
		return rs, nil
	}

	injectSensor2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return nil, errors.New("can't get readings")
	}

	t.Run("GetReadings", func(t *testing.T) {
		expected := map[string]*structpb.Value{}
		for k, v := range rs {
			vv, err := structpb.NewValue(v)
			test.That(t, err, test.ShouldBeNil)
			expected[k] = vv
		}
		extra, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)

		resp, err := sensorServer.GetReadings(context.Background(), &pb.GetReadingsRequest{Name: testSensorName, Extra: extra})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Readings, test.ShouldResemble, expected)
		test.That(t, extraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = sensorServer.GetReadings(context.Background(), &pb.GetReadingsRequest{Name: failSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get readings")

		_, err = sensorServer.GetReadings(context.Background(), &pb.GetReadingsRequest{Name: fakeSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a generic sensor")

		_, err = sensorServer.GetReadings(context.Background(), &pb.GetReadingsRequest{Name: missingSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no generic sensor")
	})
}
