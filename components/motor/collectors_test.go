package motor

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	tu "go.viam.com/rdk/testutils"
)

const (
	componentName   = "motor"
	captureInterval = time.Second
)

func TestMotorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      "Motor position collector should write a position response",
			collector: newPositionCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
				Position: 1.0,
			}),
		},
		{
			name:      "Motor isPowered collector should write an isPowered response",
			collector: newIsPoweredCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.IsPoweredResponse{
				IsOn:     false,
				PowerPct: .5,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			params := data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
				Clock:         mockClock,
				Target:        &buf,
			}

			motor := newMotor()
			col, err := tc.collector(motor, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			test.That(t, buf.Length(), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

type fakeMotor struct {
	Motor
}

func newMotor() Motor {
	return &fakeMotor{}
}

func (m *fakeMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 1.0, nil
}

func (m *fakeMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return false, .5, nil
}
