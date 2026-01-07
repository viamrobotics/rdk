package board

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/board/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	// we need analog pins that are readers, not writers,
	// to collect data from.
	analogReaderNameKey        = "reader_name"
	gpioPinNameKey             = "pin_name"
	analogs             method = iota
	gpios
	doCommand
)

func (m method) String() string {
	if m == analogs {
		return "Analogs"
	}
	if m == gpios {
		return "Gpios"
	}
	if m == doCommand {
		return "DoCommand"
	}
	return "Unknown"
}

// newAnalogCollector returns a collector to register an analog reading method. If one is already registered
// with the same MethodMetadata it will panic.
func newAnalogCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		var analogValue AnalogValue

		analogReaderNameMarshaled, ok := arg[analogReaderNameKey]
		if !ok {
			return res, data.NewFailedToReadError(params.ComponentName, analogs.String(),
				errors.New("Must supply reader_name in additional_params for analog collector"))
		}

		analogReaderName, err := unmarshalName(analogReaderNameMarshaled)
		if err != nil {
			return res, data.NewFailedToReadError(params.ComponentName, analogs.String(), errors.Wrap(err, "failed to get reader name"))
		}

		if reader, err := board.AnalogByName(analogReaderName); err == nil {
			analogValue, err = reader.Read(ctx, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if data.IsNoCaptureToStoreError(err) {
					return res, err
				}
				return res, data.NewFailedToReadError(params.ComponentName, analogs.String(), err)
			}
		}

		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.ReadAnalogReaderResponse{
			Value:    int32(analogValue.Value),
			MinRange: analogValue.Min,
			MaxRange: analogValue.Max,
			StepSize: analogValue.StepSize,
		})
	})
	return data.NewCollector(cFunc, params)
}

// newGPIOCollector returns a collector to register a gpio get method. If one is already registered
// with the same MethodMetadata it will panic.
func newGPIOCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		var value bool

		pinNameMarshaled, ok := arg[gpioPinNameKey]
		if !ok {
			return res, data.NewFailedToReadError(params.ComponentName, gpios.String(),
				errors.New("Must supply pin_name in additional params for gpio collector"))
		}

		pinName, err := unmarshalName(pinNameMarshaled)
		if err != nil {
			return res, data.NewFailedToReadError(
				params.ComponentName, gpios.String(),
				errors.Wrap(err, fmt.Sprintf("failed to get pin name: %v; type: %s", pinNameMarshaled, pinNameMarshaled.TypeUrl)),
			)
		}

		if gpio, err := board.GPIOPinByName(pinName); err == nil {
			value, err = gpio.Get(ctx, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if data.IsNoCaptureToStoreError(err) {
					return res, err
				}
				return res, data.NewFailedToReadError(params.ComponentName, gpios.String(), err)
			}
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetGPIOResponse{
			High: value,
		})
	})
	return data.NewCollector(cFunc, params)
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(board, params)
	return data.NewCollector(cFunc, params)
}

func assertBoard(resource interface{}) (Board, error) {
	board, ok := resource.(Board)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return board, nil
}

func unmarshalName(nameMarshaled *anypb.Any) (string, error) {
	var structVal structpb.Value
	if err := nameMarshaled.UnmarshalTo(&structVal); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal name")
	}

	switch kind := structVal.Kind.(type) {
	case *structpb.Value_StringValue:
		return kind.StringValue, nil
	default:
		return "", errors.New("name must be a string")
	}
}
