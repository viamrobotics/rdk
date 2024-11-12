package powersensor_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

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
	expected1Struct, err := structpb.NewValue(map[string]any{
		"volts": 1.0,
		"is_ac": false,
	})
	test.That(t, err, test.ShouldBeNil)

	expected2Struct, err := structpb.NewValue(map[string]any{
		"amperes": 1.0,
		"is_ac":   false,
	})
	test.That(t, err, test.ShouldBeNil)

	expected3Struct, err := structpb.NewValue(map[string]any{
		"watts": 1.0,
	})
	test.That(t, err, test.ShouldBeNil)

	expected4Struct, err := structpb.NewValue(map[string]any{
		"readings": map[string]any{
			"reading1": false,
			"reading2": "test",
		},
	})
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  *datasyncpb.SensorData
	}{
		{
			name:      "Power sensor voltage collector should write a voltage response",
			collector: powersensor.NewVoltageCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data:     &datasyncpb.SensorData_Struct{Struct: expected1Struct.GetStructValue()},
			},
		},
		{
			name:      "Power sensor current collector should write a current response",
			collector: powersensor.NewCurrentCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data:     &datasyncpb.SensorData_Struct{Struct: expected2Struct.GetStructValue()},
			},
		},
		{
			name:      "Power sensor power collector should write a power response",
			collector: powersensor.NewPowerCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data:     &datasyncpb.SensorData_Struct{Struct: expected3Struct.GetStructValue()},
			},
		},
		{
			name:      "Power sensor readings collector should write a readings response",
			collector: powersensor.NewReadingsCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data:     &datasyncpb.SensorData_Struct{Struct: expected4Struct.GetStructValue()},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			buf := tu.NewMockBuffer(ctx)
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

			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, tc.expected)
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
