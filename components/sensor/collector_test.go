package sensor

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/sensor/v1"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
)

const componentName = "sensor"

var readingMap = map[string]any{"reading1": false, "reading2": "test"}

func TestSensorCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: componentName,
		Interval:      time.Second,
		Logger:        golog.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	sensor := newSensor(componentName)
	col, err := newSensorCollector(sensor, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()
	mockClock.Add(1 * time.Second)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(buf.Writes), test.ShouldEqual, 1)
	test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, toProtoMap(getExpectedMap(readingMap)))
}

type fakeSensor struct {
	Sensor
	name resource.Name
}

func newSensor(name string) Sensor {
	return &fakeSensor{name: resource.Name{Name: name}}
}

func (s *fakeSensor) Name() resource.Name {
	return s.name
}

func (s *fakeSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return readingMap, nil
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

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
