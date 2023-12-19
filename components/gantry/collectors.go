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

// newPositionCollector returns a collector to register a position method. If one is already registered
// with the same MethodMetadata it will panic.
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
		return pb.GetPositionResponse{
			PositionsMm: scaleMetersToMm(v),
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

// newLengthsCollector returns a collector to register a lengths method. If one is already registered
// with the same MethodMetadata it will panic.
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
		return pb.GetLengthsResponse{
			LengthsMm: scaleMetersToMm(v),
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func scaleMetersToMm(meters []float64) []float64 {
	ret := make([]float64, len(meters))
	for i := range ret {
		ret[i] = meters[i] * 1000
	}
	return ret
}

func assertGantry(resource interface{}) (Gantry, error) {
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return gantry, nil
}
