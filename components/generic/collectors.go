package generic

import (
	"context"
	"errors"
	"time"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
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
func methodParamsFromProto(proto map[string]*anypb.Any) (map[string]interface{}, error) {
	methodParameters := make(map[string]interface{})

	// logger := logging.NewLogger("test")
	for key, value := range proto {
		if value == nil {
			methodParameters[key] = nil
		}
		// structValue := &structpb.Value_StringValue{}
		// if err := value.(structValue); err != nil {
		// 	logger.Info(value.TypeUrl)
		// 	return nil, err
		// }
		methodParameters[key] = string(value.GetValue())
	}

	return methodParameters, nil
}

// newCommandCollector returns a collector to register a sensor reading method. If one is already registered
// with the same MethodMetadata it will panic.
func newCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	reso, err := assertResource(resource)
	if err != nil {
		return nil, err
	}

	logger := logging.NewLogger("test")
	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		var s structpb.Struct
		if err := params.MethodParams["docommand_payload"].UnmarshalTo(&s); err != nil {
			logger.Info(err)
			return res, err
		}

		logger.Info(s.AsMap())

		payload := s.AsMap()
		if err != nil {
			return res, err
		}
		logger.Infof("capturing docommand with payload %#v\n", payload)

		values, err := reso.DoCommand(ctx, payload)

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

type Resource interface {
	resource.Resource
}

func assertResource(resource interface{}) (Resource, error) {
	res, ok := resource.(Resource)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return res, nil
}
