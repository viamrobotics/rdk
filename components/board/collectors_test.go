package board_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/protobuf/ptypes/wrappers"
	datasyncpb "go.viam.com/api/app/datasync/v1"
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
	captureInterval = time.Millisecond
)

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
	}{
		{
			name: "Board analog collector should write an analog response",
			params: data.CollectorParams{
				DataType:      data.CaptureTypeTabular,
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				MethodParams: map[string]*anypb.Any{
					"reader_name": convertInterfaceToAny("analog"),
				},
			},
			collector: board.NewAnalogCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"value":     1,
					"min_range": 0,
					"max_range": 10,
					"step_size": float64(float32(0.1)),
				})},
			}},
		},
		{
			name: "Board gpio collector should write a gpio response",
			params: data.CollectorParams{
				DataType:      data.CaptureTypeTabular,
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				MethodParams: map[string]*anypb.Any{
					"pin_name": convertInterfaceToAny("gpio"),
				},
			},
			collector: board.NewGPIOCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"high": true,
				})},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			tc.params.Clock = clock.New()
			tc.params.Target = buf

			board := newBoard()
			col, err := tc.collector(board, tc.params)
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
