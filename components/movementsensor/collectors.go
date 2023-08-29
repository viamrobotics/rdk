package movementsensor

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

// TODO: add tests for this file.

func assertMovementSensor(resource interface{}) (MovementSensor, error) {
	ms, ok := resource.(MovementSensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return ms, nil
}

type lowLevelCollector func(ctx context.Context, ms MovementSensor, extra map[string]interface{}) (interface{}, error)

func registerCollector(name string, f lowLevelCollector) {
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: name,
	}, func(resource interface{}, params data.CollectorParams) (data.Collector, error) {
		ms, err := assertMovementSensor(resource)
		if err != nil {
			return nil, err
		}

		cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
			v, err := f(ctx, ms, data.FromDMExtraMap)
			if err != nil {
				// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
				// is used in the datamanager to exclude readings from being captured and stored.
				if errors.Is(err, data.ErrNoCaptureToStore) {
					return nil, err
				}
				return nil, data.FailedToReadErr(params.ComponentName, name, err)
			}
			return v, nil
		})
		return data.NewCollector(cFunc, params)
	},
	)
}
