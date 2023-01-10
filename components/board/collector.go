package board

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	analogs method = iota
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

type analogRecords struct {
	Readings []analogRecord
}

type analogRecord struct {
	AnalogName  string
	AnalogValue int
}

type gpioRecords struct {
	Readings []gpioRecord
}

type gpioRecord struct {
	GPIOName  string
	GPIOValue bool
}

func newAnalogCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		var readings []analogRecord
		for k := range arg {
			if reader, ok := board.AnalogReaderByName(k); ok {
				value, err := reader.Read(ctx, nil)
				if err != nil {
					return nil, data.FailedToReadErr(params.ComponentName, analogs.String(), err)
				}
				readings = append(readings, analogRecord{AnalogName: k, AnalogValue: value})
			}
		}
		return analogRecords{Readings: readings}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newGPIOCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		var readings []gpioRecord
		for k := range arg {
			if gpio, err := board.GPIOPinByName(k); err == nil {
				value, err := gpio.Get(ctx, nil)
				if err != nil {
					return nil, data.FailedToReadErr(params.ComponentName, gpios.String(), err)
				}
				readings = append(readings, gpioRecord{GPIOName: k, GPIOValue: value})
			}
		}
		return gpioRecords{Readings: readings}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertBoard(resource interface{}) (Board, error) {
	board, ok := resource.(Board)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}

	return board, nil
}
