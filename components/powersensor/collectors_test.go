package powersensor

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/powersensor/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
)

type collectorFunc func(resource interface{}, params data.CollectorParams) (data.Collector, error)

const (
	componentName   = "powersensor"
	captureInterval = time.Second
)

func TestPowerSensorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector collectorFunc
		expected  map[string]any
	}{
		{
			name: "Power sensor voltage collector should write a voltage response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newVoltageCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetVoltageResponse{
				Volts: 1.0,
				IsAc:  false,
			}),
		},
		{
			name: "Power sensor current collector should write a current response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newCurrentCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetCurrentResponse{
				Amperes: 1.0,
				IsAc:    true,
			}),
		},
		{
			name: "Power sensor power collector should write a power response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newPowerCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetPowerResponse{
				Watts: 1.0,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			pwrSens := newPowerSensor(componentName)
			col, err := tc.collector(pwrSens, tc.params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			test.That(t, buf.Length(), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

type fakePowerSensor struct {
	PowerSensor
	name resource.Name
}

func newPowerSensor(name string) PowerSensor {
	return &fakePowerSensor{name: resource.Name{Name: name}}
}

func (i *fakePowerSensor) Name() resource.Name {
	return i.name
}

func (i *fakePowerSensor) Voltage(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return 1.0, false, nil
}

func (i *fakePowerSensor) Current(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return 1.0, true, nil
}

func (i *fakePowerSensor) Power(ctx context.Context, cmd map[string]interface{}) (float64, error) {
	return 1.0, nil
}
