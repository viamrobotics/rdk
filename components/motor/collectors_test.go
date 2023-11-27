package motor_test

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "motor"
	captureInterval = time.Second
	numRetries      = 5
)

func TestMotorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      "Motor position collector should write a position response",
			collector: motor.NewPositionCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
				Position: 1.0,
			}),
		},
		{
			name:      "Motor isPowered collector should write an isPowered response",
			collector: motor.NewIsPoweredCollector,
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
				Logger:        logging.NewTestLogger(t),
				Clock:         mockClock,
				Target:        &buf,
			}

			motor := newMotor()
			col, err := tc.collector(motor, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			tu.Retry(func() bool {
				return buf.Length() != 0
			}, numRetries)
			test.That(t, buf.Length(), test.ShouldBeGreaterThan, 0)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

func newMotor() motor.Motor {
	m := &inject.Motor{}
	m.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return false, .5, nil
	}
	m.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 1.0, nil
	}
	return m
}
