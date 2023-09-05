package encoder

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	ticksCount method = iota
)

func (m method) String() string {
	if m == ticksCount {
		return "TicksCount"
	}
	return "Unknown"
}

// Ticks wraps the returned ticks value.
type Ticks struct {
	Ticks int64
}

func newTicksCountCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	encoder, err := assertEncoder(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, _, err := encoder.Position(ctx, PositionTypeUnspecified, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, ticksCount.String(), err)
		}
		return Ticks{Ticks: int64(v)}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertEncoder(resource interface{}) (Encoder, error) {
	encoder, ok := resource.(Encoder)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return encoder, nil
}
