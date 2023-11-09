package sensor_test

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	pb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const captureInterval = time.Second

var readingMap = map[string]any{"reading1": false, "reading2": "test"}

func TestSensorCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: "sensor",
		Interval:      captureInterval,
		Logger:        logging.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	sens := newSensor()
	col, err := sensor.NewSensorCollector(sens, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()
	mockClock.Add(captureInterval)

	test.That(t, buf.Length(), test.ShouldEqual, 1)
	test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tu.ToProtoMapIgnoreOmitEmpty(getExpectedMap(readingMap)))
}

func newSensor() sensor.Sensor {
	s := &inject.Sensor{}
	s.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return readingMap, nil
	}
	return s
}

func getExpectedMap(data map[string]any) pb.GetReadingsResponse {
	readings := make(map[string]*structpb.Value)
	for name, value := range data {
		val, _ := structpb.NewValue(value)
		readings[name] = val
	}
	return pb.GetReadingsResponse{
		Readings: readings,
	}
}
