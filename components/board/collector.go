package board

import (
	"context"

	"github.com/pkg/errors"
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

// AnalogRecords a collection of AnalogRecord.
type AnalogRecords struct {
	Readings []AnalogRecord
}

// AnalogRecord a single analog reading.
type AnalogRecord struct {
	AnalogName  string
	AnalogValue int
}

// GpioRecords a collection of GpioRecord.
type GpioRecords struct {
	Readings []GpioRecord
}

// GpioRecord a signle gpio reading.
type GpioRecord struct {
	GPIOName  string
	GPIOValue bool
}

func newAnalogCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		var readings []AnalogRecord
		for k := range arg {
			if reader, ok := board.AnalogReaderByName(k); ok {
				value, err := reader.Read(ctx, data.FromDMExtraMap)
				if err != nil {
					// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
					// is used in the datamanager to exclude readings from being captured and stored.
					if errors.Is(err, data.ErrNoCaptureToStore) {
						return nil, err
					}
					return nil, data.FailedToReadErr(params.ComponentName, analogs.String(), err)
				}
				readings = append(readings, AnalogRecord{AnalogName: k, AnalogValue: value})
			}
		}
		return AnalogRecords{Readings: readings}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newGPIOCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	board, err := assertBoard(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		var readings []GpioRecord
		for k := range arg {
			if gpio, err := board.GPIOPinByName(k); err == nil {
				value, err := gpio.Get(ctx, data.FromDMExtraMap)
				if err != nil {
					// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
					// is used in the datamanager to exclude readings from being captured and stored.
					if errors.Is(err, data.ErrNoCaptureToStore) {
						return nil, err
					}
					return nil, data.FailedToReadErr(params.ComponentName, gpios.String(), err)
				}
				readings = append(readings, GpioRecord{GPIOName: k, GPIOValue: value})
			}
		}
		return GpioRecords{Readings: readings}, nil
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
