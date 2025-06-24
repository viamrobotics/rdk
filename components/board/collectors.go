package board

import (
	"context"
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
	return ""
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
		if _, ok := arg[analogReaderNameKey]; !ok {
			return res, data.NewFailedToReadError(params.ComponentName, analogs.String(),
				errors.New("Must supply reader_name in additional_params for analog collector"))
		}
		if reader, err := board.AnalogByName(arg[analogReaderNameKey].String()); err == nil {
			analogValue, err = reader.Read(ctx, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if errors.Is(err, data.ErrNoCaptureToStore) {
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
		if _, ok := arg[gpioPinNameKey]; !ok {
			return res, data.NewFailedToReadError(params.ComponentName, gpios.String(),
				errors.New("Must supply pin_name in additional params for gpio collector"))
		}
		if gpio, err := board.GPIOPinByName(arg[gpioPinNameKey].String()); err == nil {
			value, err = gpio.Get(ctx, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if errors.Is(err, data.ErrNoCaptureToStore) {
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

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		var payload map[string]interface{}

		if payloadAny, exists := params.MethodParams["docommand_input"]; exists && payloadAny != nil {
			if payloadAny.MessageIs(&structpb.Struct{}) {
				var s structpb.Struct
				if err := payloadAny.UnmarshalTo(&s); err != nil {
					return res, err
				}
				payload = s.AsMap()
			} else {
				// handle empty payload
				payload = make(map[string]interface{})
			}
		} else {
			// key does not exist
			return res, errors.New("missing payload")
		}

		values, err := board.DoCommand(ctx, payload)
		if err != nil {
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, "DoCommand", err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, values)
	})
	return data.NewCollector(cFunc, params)
}

func assertBoard(resource interface{}) (Board, error) {
	board, ok := resource.(Board)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return board, nil
}
