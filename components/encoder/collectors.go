package encoder

import (
	"context"

	"go.viam.com/rdk/data"
	"google.golang.org/protobuf/types/known/anypb"
)

type method int64

const (
	getTicksCount method = iota
)

func (m method) String() string {
	switch m {
	case getTicksCount:
		return "GetTicksCount"
	}
	return "Unknown"
}

// Ticks wraps the returned ticks value.
type Ticks struct {
	Ticks int64
}

func newGetTicksCountCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	encoder, err := assertEncoder(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := encoder.GetTicksCount(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getTicksCount.String(), err)
		}
		return Ticks{Ticks: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertEncoder(resource interface{}) (Encoder, error) {
	encoder, ok := resource.(Encoder)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return encoder, nil
}
