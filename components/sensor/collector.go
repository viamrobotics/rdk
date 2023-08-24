package sensor

import (
	"context"
	"errors"

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

// ReadingRecords a collection of ReadingRecord.
type ReadingRecords struct {
	Readings []ReadingRecord
}

// ReadingRecord a single analog reading.
type ReadingRecord struct {
	ReadingName string
	Reading     interface{}
}

func newSensorCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	sensorResource, err := assertSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
		var records []ReadingRecord
		values, err := sensorResource.Readings(ctx, nil) // TODO (RSDK-1972): pass in something here from the config rather than nil?
		if err != nil {
			// If err is from a modular filter component, propagate it to getAndPushNextReading().
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}
		for name, value := range values {
			if len(arg) != 0 {
				// If specific sensor reading names were passed in the robot config, report only those
				// Otherwise, report all sensor values
				if _, ok := arg[name]; !ok {
					continue
				}
			}
			records = append(records, ReadingRecord{ReadingName: name, Reading: value})
		}
		return ReadingRecords{Readings: records}, nil
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
