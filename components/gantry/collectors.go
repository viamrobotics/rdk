package gantry

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	position method = iota
	lengths
)

func (m method) String() string {
	switch m {
	case position:
		return "Position"
	case lengths:
		return "Lengths"
	}
	return "Unknown"
}

// Position wraps the returned position values.
type Position struct {
	Position []float64
}

func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		v, err := gantry.Position(ctx, nil)
		if err != nil {
			// If err is from a modular filter component, propagate it to getAndPushNextReading().
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		return Position{Position: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

// Lengths wraps the returns lengths values.
type Lengths struct {
	Lengths []float64
}

func newLengthsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		v, err := gantry.Lengths(ctx, nil)
		if err != nil {
			// If err is from a modular filter component, propagate it to getAndPushNextReading().
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, lengths.String(), err)
		}
		return Lengths{Lengths: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertGantry(resource interface{}) (Gantry, error) {
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return gantry, nil
}
