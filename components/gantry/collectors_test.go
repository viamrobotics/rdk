package gantry

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/gantry/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	tu "go.viam.com/rdk/testutils"
)

const (
	componentName   = "gantry"
	captureInterval = time.Second
)

var floatList = []float64{1.0, 2.0, 3.0}

func TestGantryCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      "Length collector should write a lengths response",
			collector: newLengthsCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetLengthsResponse{
				LengthsMm: scaleMetersToMm(floatList),
			}),
		},
		{
			name:      "End position collector should write a list of positions",
			collector: newPositionCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
				PositionsMm: scaleMetersToMm(floatList),
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

			gantry := newGantry()
			col, err := tc.collector(gantry, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			test.That(t, buf.Length(), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

type fakeGantry struct {
	Gantry
}

func newGantry() Gantry {
	return &fakeGantry{}
}

func (g *fakeGantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return floatList, nil
}

func (g *fakeGantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return floatList, nil
}
