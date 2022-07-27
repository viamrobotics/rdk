package gantry

import (
	"context"

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

func newGetPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gantry.GetPosition(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getPosition.String(), err)
		}
		return PositionWrapper{Position: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

// LengthsWrapper wraps the returns lengths values.
type LengthsWrapper struct {
	Lengths []float64
}

func newGetLengthsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gantry.GetLengths(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getLengths.String(), err)
		}
		return LengthsWrapper{Lengths: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertGantry(resource interface{}) (Gantry, error) {
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return gantry, nil
}
