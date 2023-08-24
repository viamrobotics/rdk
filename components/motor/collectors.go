package motor

import (
	"context"
	"errors"

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
		return "Position"
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
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		v, err := motor.Position(ctx, nil)
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

// Powered wraps the returned IsPowered value.
type Powered struct {
	IsPowered bool
	PowerPct  float64
}

func newIsPoweredCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		v, powerPct, err := motor.IsPowered(ctx, nil)
		if err != nil {
			// If err is from a modular filter component, propagate it to getAndPushNextReading().
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, isPowered.String(), err)
		}
		return Powered{IsPowered: v, PowerPct: powerPct}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertMotor(resource interface{}) (Motor, error) {
	motor, ok := resource.(Motor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return motor, nil
}
