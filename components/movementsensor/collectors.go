package movementsensor

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

// TODO: add tests for this file.

func assertMovementSensor(resource interface{}) (MovementSensor, error) {
	ms, ok := resource.(MovementSensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return ms, nil
}

type lowLevelCollector func(ctx context.Context, ms MovementSensor) (interface{}, error)

func registerCollector(name string, f lowLevelCollector) {
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: name,
	}, func(resource interface{}, params data.CollectorParams) (data.Collector, error) {
		ms, err := assertMovementSensor(resource)
		if err != nil {
			return nil, err
		}

		cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
			v, err := f(ctx, ms)
			if err != nil {
				return nil, data.FailedToReadErr(params.ComponentName, name, err)
			}
			return v, nil
		})
		return data.NewCollector(cFunc, params)
	},
	)
}
