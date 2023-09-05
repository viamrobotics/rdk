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
		var records []ReadingRecord
		values, err := sensorResource.Readings(ctx, data.FromDMExtraMap) // TODO (RSDK-1972): pass in something here from the config?
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
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
