package motor

import (
	"context"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getPosition method = iota
	isPowered
)

func (m method) String() string {
	switch m {
	case getPosition:
		return "GetPosition"
	case isPowered:
		return "IsPowered"
	}
	return "Unknown"
}

// Position wraps the returned position value.
type Position struct {
	Revolutions float64
}

func newGetPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := motor.GetPosition(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getPosition.String(), err)
		}
		return Position{Revolutions: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

// Powered wraps the returned IsPowered value.
type Powered struct {
	Value bool
}

func newIsPoweredCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := motor.IsPowered(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, isPowered.String(), err)
		}
		return Powered{Value: v}, nil
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
