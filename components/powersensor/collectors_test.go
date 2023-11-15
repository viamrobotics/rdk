package powersensor_test

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	pb "go.viam.com/api/component/powersensor/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "powersensor"
	captureInterval = time.Second
)

var readingMap = map[string]any{"reading1": false, "reading2": "test"}

func TestPowerSensorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      "Power sensor voltage collector should write a voltage response",
			collector: powersensor.NewVoltageCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetVoltageResponse{
				Volts: 1.0,
				IsAc:  false,
			}),
		},
		{
			name:      "Power sensor current collector should write a current response",
			collector: powersensor.NewCurrentCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetCurrentResponse{
				Amperes: 1.0,
				IsAc:    false,
			}),
		},
		{
			name:      "Power sensor power collector should write a power response",
			collector: powersensor.NewPowerCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetPowerResponse{
				Watts: 1.0,
			}),
		},
		{
			name:      "Power sensor readings collector should write a readings response",
			collector: powersensor.NewReadingsCollector,
			expected:  tu.ToProtoMapIgnoreOmitEmpty(data.GetExpectedReadingsStruct(readingMap).AsMap()),
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

			pwrSens := newPowerSensor()
			col, err := tc.collector(pwrSens, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			test.That(t, buf.Length(), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

func newPowerSensor() powersensor.PowerSensor {
	p := &inject.PowerSensor{}
	p.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 1.0, false, nil
	}
	p.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 1.0, false, nil
	}
	p.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 1.0, nil
	}
	p.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return readingMap, nil
	}
	return p
}
