package motor

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
)

type collectorFunc func(resource interface{}, params data.CollectorParams) (data.Collector, error)

const componentName = "motor"

func TestMotorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector collectorFunc
		expected  map[string]any
	}{
		{
			name: "Motor position collector should write a position response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newPositionCollector,
			expected: toProtoMap(pb.GetPositionResponse{
				Position: 1.0,
			}),
		},
		{
			name: "Motor isPowered collector should write an isPowered response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newIsPoweredCollector,
			expected: toProtoMap(pb.IsPoweredResponse{
				IsOn:     false,
				PowerPct: .5,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			motor := newMotor(componentName)
			col, err := tc.collector(motor, tc.params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(1 * time.Second)

			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(buf.Writes), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

type fakeMotor struct {
	Motor
	name resource.Name
}

func newMotor(name string) Motor {
	return &fakeMotor{name: resource.Name{Name: name}}
}

func (m *fakeMotor) Name() resource.Name {
	return m.name
}

func (m *fakeMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 1.0, nil
}

func (m *fakeMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return false, .5, nil
}

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
