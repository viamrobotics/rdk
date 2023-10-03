package gantry

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/gantry/v1"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
)

type collectorFunc func(resource interface{}, params data.CollectorParams) (data.Collector, error)

const componentName = "gantry"

var floatList = []float64{1.0, 2.0, 3.0}

func TestGantryCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector collectorFunc
		expected  map[string]any
	}{
		{
			name: "Length collector should write a lengths response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newLengthsCollector,
			expected: toProtoMap(pb.GetLengthsResponse{
				LengthsMm: scaleMetersToMm(floatList),
			}),
		},
		{
			name: "End position collector should write a list of positions",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newPositionCollector,
			expected: toProtoMap(pb.GetPositionResponse{
				PositionsMm: scaleMetersToMm(floatList),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			gantry := newGantry(componentName)
			col, err := tc.collector(gantry, tc.params)
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

type fakeGantry struct {
	Gantry
	name resource.Name
}

func newGantry(name string) Gantry {
	return &fakeGantry{name: resource.Name{Name: name}}
}

func (g *fakeGantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return floatList, nil
}

func (g *fakeGantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return floatList, nil
}

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
