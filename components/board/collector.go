package board

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/board/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	analogReaderNameKey        = "reader_name"
	gpioPinNameKey             = "pin_name"
	analogs             method = iota
	gpios
)

func (m method) String() string {
	if m == analogs {
		return "Analogs"
	}
	if m == gpios {
		return "Gpios"
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

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		var value int
		if _, ok := arg[analogReaderNameKey]; !ok {
			return nil, data.FailedToReadErr(params.ComponentName, analogs.String(),
				errors.New("Must supply reader_name in additional_params for analog collector"))
		}
		if reader, ok := board.AnalogReaderByName(arg[analogReaderNameKey].String()); ok {
			value, err = reader.Read(ctx, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if errors.Is(err, data.ErrNoCaptureToStore) {
					return nil, err
				}
				return nil, data.FailedToReadErr(params.ComponentName, analogs.String(), err)
			}
		}
		return pb.ReadAnalogReaderResponse{
			Value: int32(value),
		}, nil
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

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		var value bool
		if _, ok := arg[gpioPinNameKey]; !ok {
			return nil, data.FailedToReadErr(params.ComponentName, gpios.String(),
				errors.New("Must supply pin_name in additional params for gpio collector"))
		}
		if gpio, err := board.GPIOPinByName(arg[gpioPinNameKey].String()); err == nil {
			value, err = gpio.Get(ctx, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if errors.Is(err, data.ErrNoCaptureToStore) {
					return nil, err
				}
				return nil, data.FailedToReadErr(params.ComponentName, gpios.String(), err)
			}
		}
		return pb.GetGPIOResponse{
			High: value,
		}, nil
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
