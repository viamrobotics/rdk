package motor

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	position method = iota
	isPowered
)

func (m method) String() string {
	switch m {
	case position:
		return "GetPosition"
	case isPowered:
		return "IsPowered"
	}
	return "Unknown"
}

// Position wraps the returned position value.
type Position struct {
	Position float64
}

func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := motor.Position(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		return Position{Position: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

// Powered wraps the returned IsPowered value.
type Powered struct {
	IsPowered bool
}

func newIsPoweredCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := motor.IsPowered(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, isPowered.String(), err)
		}
		return Powered{IsPowered: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertMotor(resource interface{}) (Motor, error) {
	motor, ok := resource.(Motor)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return motor, nil
}
