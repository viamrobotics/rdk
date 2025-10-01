package board_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/data"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "board"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

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
					"reader_name": convertStringToAny("analog"),
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
					"pin_name": convertStringToAny("gpio"),
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

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       board.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newBoard() },
	})
}

func newBoard() board.Board {
	b := &inject.Board{}
	analog := &inject.Analog{}
	analog.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
		return board.AnalogValue{Value: 1, Min: 0, Max: 10, StepSize: 0.1}, nil
	}
	b.AnalogByNameFunc = func(name string) (board.Analog, error) {
		if name == "analog" {
			return analog, nil
		}
		return nil, errors.New("analog not found")
	}
	gpioPin := &inject.GPIOPin{}
	gpioPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, nil
	}
	b.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		if name == "gpio" {
			return gpioPin, nil
		}
		return nil, errors.New("gpio pin not found")
	}
	b.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return b
}

func convertStringToAny(str string) *anypb.Any {
	anyValue := &anypb.Any{}

	// Handle string values as structpb.Value with StringValue
	structValue := &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: str,
		},
	}

	anypb.MarshalFrom(anyValue, structValue, proto.MarshalOptions{})

	return anyValue
}
