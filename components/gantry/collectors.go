package gantry

import (
	"context"
	"errors"

	pb "go.viam.com/api/component/gantry/v1"
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
		v, err := gantry.Position(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		// Done to scale position from meters to mm
		for i := range v {
			v[i] *= 1000
		}
		return pb.GetPositionResponse{
			PositionsMm: v,
		}, nil
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
		v, err := gantry.Lengths(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, lengths.String(), err)
		}
		// Done to scale length from meters to mm
		for i := range v {
			v[i] *= 1000
		}
		return pb.GetLengthsResponse{
			LengthsMm: v,
		}, nil
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
