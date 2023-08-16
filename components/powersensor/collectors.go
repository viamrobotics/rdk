package powersensor

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

func assertPowerSensor(resource interface{}) (PowerSensor, error) {
	ps, ok := resource.(PowerSensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return ps, nil
}

type lowLevelCollector func(ctx context.Context, ps PowerSensor) (interface{}, error)

func registerCollector(name string, f lowLevelCollector) {
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: name,
	}, func(resource interface{}, params data.CollectorParams) (data.Collector, error) {
		ps, err := assertPowerSensor(resource)
		if err != nil {
			return nil, err
		}

		cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
			v, err := f(ctx, ps)
			if err != nil {
				return nil, data.FailedToReadErr(params.ComponentName, name, err)
			}
			return v, nil
		})
		return data.NewCollector(cFunc, params)
	},
	)
}
