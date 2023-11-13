package sensor

import (
	"context"
	"errors"

	pb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	readings method = iota
)

func (m method) String() string {
	if m == readings {
		return "Readings"
	}
	return "Unknown"
}

// NewSensorCollector returns a collector to register a sensor reading method. If one is already registered
// with the same MethodMetadata it will panic.
func NewSensorCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	sensorResource, err := assertSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		values, err := sensorResource.Readings(ctx, data.FromDMExtraMap) // TODO (RSDK-1972): pass in something here from the config?
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}
		readings := make(map[string]*structpb.Value)
		for name, value := range values {
			val, err := structpb.NewValue(value)
			if err != nil {
				return nil, err
			}
			readings[name] = val
		}
		return pb.GetReadingsResponse{
			Readings: readings,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertSensor(resource interface{}) (Sensor, error) {
	sensorResource, ok := resource.(Sensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return sensorResource, nil
}
