package board

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
)

type collectorFunc func(resource interface{}, params data.CollectorParams) (data.Collector, error)

const componentName = "board"

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector collectorFunc
		expected  map[string]any
	}{
		{
			name: "Board analog collector should write an analog response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newAnalogCollector,
			expected: toProtoMap(pb.ReadAnalogReaderResponse{
				Value: 1,
			}),
		},
		{
			name: "Board gpio collector should write a gpio response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newGPIOCollector,
			expected: toProtoMap(pb.GetGPIOResponse{
				High: true,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			board := newBoard(componentName)
			col, err := tc.collector(board, tc.params)
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

type fakeBoard struct {
	LocalBoard
	name resource.Name
}

func newBoard(name string) Board {
	return &fakeBoard{name: resource.Name{Name: name}}
}

func (b *fakeBoard) Name() resource.Name {
	return b.name
}

func (b *fakeBoard) AnalogReaderByName(name string) (AnalogReader, bool) {
	return &fakeAnalogReader{}, true
}

func (b *fakeBoard) GPIOPinByName(name string) (GPIOPin, error) {
	return &fakeGPIOPin{}, nil
}

type fakeGPIOPin struct {
	GPIOPin
}

func (gp *fakeGPIOPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return true, nil
}

type fakeAnalogReader struct {
	AnalogReader
}

func (a *fakeAnalogReader) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	return 1, nil
}

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
