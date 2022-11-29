package servo

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	position method = iota
)

func (m method) String() string {
	if m == position {
		return "Position"
	}
	return "Unknown"
}

// Position wraps the returned set angle (degrees) value.
type Position struct {
	Position uint32
}

func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	servo, err := assertServo(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := servo.Position(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		return Position{Position: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertServo(resource interface{}) (Servo, error) {
	servo, ok := resource.(Servo)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return servo, nil
}
