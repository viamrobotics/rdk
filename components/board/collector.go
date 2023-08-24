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
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		var readings []AnalogRecord
		for k := range arg {
			if reader, ok := board.AnalogReaderByName(k); ok {
				value, err := reader.Read(ctx, nil)
				if err != nil {
					// If err is from a modular filter component, propagate it to getAndPushNextReading().
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
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		var readings []GpioRecord
		for k := range arg {
			if gpio, err := board.GPIOPinByName(k); err == nil {
				value, err := gpio.Get(ctx, nil)
				if err != nil {
					// If err is from a modular filter component, propagate it to getAndPushNextReading().
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
