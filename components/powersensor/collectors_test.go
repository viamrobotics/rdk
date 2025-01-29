package powersensor_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "powersensor"
	captureInterval = time.Millisecond
)

var readingMap = map[string]any{"reading1": false, "reading2": "test"}

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
	}{
		{
			name:      "Power sensor voltage collector should write a voltage response",
			collector: powersensor.NewVoltageCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"volts": 1.0,
					"is_ac": false,
				})},
			}},
		},
		{
			name:      "Power sensor current collector should write a current response",
			collector: powersensor.NewCurrentCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
						"amperes": 1.0,
						"is_ac":   false,
					})},
				},
			},
		},
		{
			name:      "Power sensor power collector should write a power response",
			collector: powersensor.NewPowerCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"watts": 1.0,
				})},
			}},
		},
		{
			name:      "Power sensor readings collector should write a readings response",
			collector: powersensor.NewReadingsCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"readings": map[string]any{
						"reading1": false,
						"reading2": "test",
					},
				})},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      data.CaptureTypeTabular,
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
			}

			pwrSens := newPowerSensor()
			col, err := tc.collector(pwrSens, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, tc.expected)
			buf.Close()
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
