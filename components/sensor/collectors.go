package sensor

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	readings method = iota
	doCommand
	getWorldPose
)

func (m method) String() string {
	switch m {
	case readings:
		return "Readings"
	case doCommand:
		return "DoCommand"
	case getWorldPose:
		return "GetWorldPose"
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
		// merge additional params with data.FromDMExtraMap
		argMap := make(map[string]interface{})
		argMap[data.FromDMString] = true
		for k, v := range arg {
			unmarshaledValue, err := data.UnmarshalToValueOrString(v)
			if err != nil {
				return res, err
			}
			argMap[k] = unmarshaledValue
		}

		values, err := sensorResource.Readings(ctx, argMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, readings.String(), err)
		}

		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResultReadings(ts, values)
	})
	return data.NewCollector(cFunc, params)
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	sensorResource, err := assertSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(sensorResource, params)
	return data.NewCollector(cFunc, params)
}


// newGetWorldPoseCollector returns a collector to capture the sensor's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertSensor(resource); err != nil {
		return nil, err
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, err
	}
	return data.NewCollector(cFunc, params)
}

func assertSensor(resource interface{}) (Sensor, error) {
	sensorResource, ok := resource.(Sensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return sensorResource, nil
}
