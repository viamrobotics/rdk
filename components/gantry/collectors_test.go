package gantry_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "gantry"
	captureInterval = time.Millisecond
)

// floatList is a lit of floats in units of millimeters.
var floatList = []float64{1000, 2000, 3000}

func TestCollectors(t *testing.T) {
	l, err := structpb.NewList([]any{1000, 2000, 3000})
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  *datasyncpb.SensorData
	}{
		{
			name:      "Length collector should write a lengths response",
			collector: gantry.NewLengthsCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"lengths_mm": structpb.NewListValue(l),
					},
				}},
			},
		},
		{
			name:      "Position collector should write a list of positions",
			collector: gantry.NewPositionCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"positions_mm": structpb.NewListValue(l),
					},
				}},
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

			gantry := newGantry()
			col, err := tc.collector(gantry, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			tu.CheckMockBufferWrites(t, ctx, start, buf.TabularWrites, tc.expected)
		})
	}
}

func newGantry() gantry.Gantry {
	g := &inject.Gantry{}
	g.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return floatList, nil
	}
	g.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return floatList, nil
	}
	return g
}
