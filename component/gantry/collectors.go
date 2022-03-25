package gantry

import (
	"context"
	"os"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getPosition method = iota
	getLengths
)

func (m method) String() string {
	switch m {
	case getPosition:
		return "GetPosition"
	case getLengths:
		return "GetLengths"
	}
	return "Unknown"
}

// PositionWrapper wraps the returned position values.
type PositionWrapper struct {
	Position []float64
}

func newGetPositionCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, queueSize int, bufferSize int, logger golog.Logger) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gantry.GetPosition(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, getPosition.String())
		}
		return PositionWrapper{Position: v}, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, bufferSize, logger), nil
}

// LengthsWrapper wraps the returns lengths values.
type LengthsWrapper struct {
	Lengths []float64
}

func newGetLengthsCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, queueSize int, bufferSize int, logger golog.Logger) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gantry.GetLengths(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, getLengths.String())
		}
		return LengthsWrapper{Lengths: v}, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, bufferSize, logger), nil
}

func assertGantry(resource interface{}) (Gantry, error) {
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return gantry, nil
}
