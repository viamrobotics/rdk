package board_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "board"
	captureInterval = time.Second
	numRetries      = 5
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
				Logger:        logging.NewTestLogger(t),
				MethodParams: map[string]*anypb.Any{
					"reader_name": convertInterfaceToAny("analog"),
				},
			},
			collector: board.NewAnalogCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.ReadAnalogReaderResponse{
				Value:    1,
				MinRange: 0,
				MaxRange: 10,
				StepSize: 0.1,
			}),
			shouldError: false,
		},
		{
			name: "Board gpio collector should write a gpio response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				MethodParams: map[string]*anypb.Any{
					"pin_name": convertInterfaceToAny("gpio"),
				},
			},
			collector: board.NewGPIOCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetGPIOResponse{
				High: true,
			}),
			shouldError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			board := newBoard()
			col, err := tc.collector(board, tc.params)
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

func newBoard() board.Board {
	b := &inject.Board{}
	analog := &inject.Analog{}
	analog.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
		return board.AnalogValue{Value: 1, Min: 0, Max: 10, StepSize: 0.1}, nil
	}
	b.AnalogByNameFunc = func(name string) (board.Analog, error) {
		return analog, nil
	}
	gpioPin := &inject.GPIOPin{}
	gpioPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, nil
	}
	b.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		return gpioPin, nil
	}
	return b
}

func convertInterfaceToAny(v interface{}) *anypb.Any {
	anyValue := &anypb.Any{}

	bytes, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	bytesValue := &wrappers.BytesValue{
		Value: bytes,
	}

	anypb.MarshalFrom(anyValue, bytesValue, proto.MarshalOptions{})
	return anyValue
}
