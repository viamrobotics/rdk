package servo

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/servo/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
)

const componentName = "servo"

func TestServoCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: componentName,
		Interval:      time.Second,
		Logger:        golog.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	servo := newServo(componentName)
	col, err := newPositionCollector(servo, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()
	mockClock.Add(1 * time.Second)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(buf.Writes), test.ShouldEqual, 1)
	test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble,
		toProtoMap(pb.GetPositionResponse{
			PositionDeg: 1.0,
		}))
}

type fakeServo struct {
	Servo
	name resource.Name
}

func newServo(name string) Servo {
	return &fakeServo{name: resource.Name{Name: name}}
}

func (s *fakeServo) Name() resource.Name {
	return s.name
}

func (s *fakeServo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	return 1.0, nil
}

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
