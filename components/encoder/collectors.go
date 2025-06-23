package encoder

import (
	"context"
	"errors"
	"time"

	pb "go.viam.com/api/component/encoder/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	ticksCount method = iota
	doCommand
)

func (m method) String() string {
	if m == ticksCount {
		return "TicksCount"
	}
	if m == doCommand {
		return "DoCommand"
	}
	return "Unknown"
}

// newTicksCountCollector returns a collector to register a ticks count method. If one is already registered
// with the same MethodMetadata it will panic.
func newTicksCountCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	encoder, err := assertEncoder(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		v, positionType, err := encoder.Position(ctx, PositionTypeUnspecified, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, ticksCount.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetPositionResponse{
			Value:        float32(v),
			PositionType: pb.PositionType(positionType),
		})
	})
	return data.NewCollector(cFunc, params)
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	encoder, err := assertEncoder(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		var payload map[string]interface{}

		if payloadAny, exists := params.MethodParams["docommand_input"]; exists && payloadAny != nil {
			if payloadAny.MessageIs(&structpb.Struct{}) {
				var s structpb.Struct
				if err := payloadAny.UnmarshalTo(&s); err != nil {
					return res, err
				}
				payload = s.AsMap()
			} else {
				// handle empty payload
				payload = make(map[string]interface{})
			}
		} else {
			// key does not exist
			return res, errors.New("missing payload")
		}

		values, err := encoder.DoCommand(ctx, payload)
		if err != nil {
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, "DoCommand", err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, values)
	})
	return data.NewCollector(cFunc, params)
}

func assertEncoder(resource interface{}) (Encoder, error) {
	encoder, ok := resource.(Encoder)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return encoder, nil
}
