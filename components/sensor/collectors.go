package sensor

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

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

// newReadingsCollector returns a collector to register a sensor reading method. If one is already registered
// with the same MethodMetadata it will panic.
func newReadingsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	sensorResource, err := assertSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		values, err := sensorResource.Readings(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}

		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResultReadings(ts, values)
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
