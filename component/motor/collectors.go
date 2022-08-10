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

type position struct {
	revolutions float64
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
		return position{v}, nil
	})
	return data.NewCollector(cFunc, params)
}

type powered struct {
	value bool
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
		return powered{value: v}, nil
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
