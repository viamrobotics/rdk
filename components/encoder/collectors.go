package encoder

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getPosition method = iota
)

func (m method) String() string {
	if m == getPosition {
		return "GetPosition"
	}
	return "Unknown"
}

// Ticks wraps the returned ticks value.
type Ticks struct {
	Ticks int64
}

func newGetPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	encoder, err := assertEncoder(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := encoder.GetPosition(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getPosition.String(), err)
		}
		return Ticks{Ticks: int64(v)}, nil
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
