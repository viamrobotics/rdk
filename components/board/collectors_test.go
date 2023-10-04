package board

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
)

const (
	componentName   = "board"
	captureInterval = time.Second
)

func TestCollectors(t *testing.T) {
	tests := []struct {
		name          string
		params        data.CollectorParams
		collector     data.CollectorConstructor
		expected      map[string]any
		shouldError   bool
		expectedError error
	}{
		{
			name: "Board analog collector should write an analog response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
				MethodParams: map[string]*anypb.Any{
					"reader_name": convertInterfaceToAny("analog"),
				},
			},
			collector: newAnalogCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.ReadAnalogReaderResponse{
				Value: 1,
			}),
			shouldError: false,
		},
		// {
		// 	name: "Board analog collector without a reader_name should error",
		// 	params: data.CollectorParams{
		// 		ComponentName: componentName,
		// 		Interval:      captureInterval,
		// 		Logger:        golog.NewTestLogger(t),
		// 	},
		// 	collector:   newAnalogCollector,
		// 	shouldError: true,
		// 	expectedError: data.FailedToReadErr(componentName, analogs.String(),
		// 		errors.New("Must supply reader_name for analog collector")),
		// },
		{
			name: "Board gpio collector should write a gpio response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
				MethodParams: map[string]*anypb.Any{
					"reader_name": convertInterfaceToAny("gpio"),
				},
			},
			collector: newGPIOCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetGPIOResponse{
				High: true,
			}),
			shouldError: false,
		},
		// {
		// 	name: "Board gpio collector without a reader_name should error",
		// 	params: data.CollectorParams{
		// 		ComponentName: componentName,
		// 		Interval:      captureInterval,
		// 		Logger:        golog.NewTestLogger(t),
		// 	},
		// 	collector:   newGPIOCollector,
		// 	shouldError: true,
		// 	expectedError: data.FailedToReadErr(componentName, gpios.String(),
		// 		errors.New("Must supply reader_name for gpio collector")),
		// },
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
			mockClock.Add(captureInterval)
			test.That(t, buf.Length(), test.ShouldEqual, 1)
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

func convertInterfaceToAny(v interface{}) *anypb.Any {
	anyValue := &anypb.Any{}
	bytes, _ := json.Marshal(v)
	bytesValue := &wrappers.BytesValue{
		Value: bytes,
	}
	//nolint:errcheck
	anypb.MarshalFrom(anyValue, bytesValue, proto.MarshalOptions{})
	return anyValue
}
