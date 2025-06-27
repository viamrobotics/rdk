package button

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	doCommand method = iota
)

func (m method) String() string {
	if m == doCommand {
		return "DoCommand"
	}
	return ""
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	button, err := assertButton(resource)
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

		values, err := button.DoCommand(ctx, payload)
		if err != nil {
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, "DoCommand", err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResultDoCommand(ts, values)
	})
	return data.NewCollector(cFunc, params)
}

func assertButton(resource interface{}) (Button, error) {
	button, ok := resource.(Button)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return button, nil
}
