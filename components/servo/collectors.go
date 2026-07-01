package servo

import (
	"context"
	"time"

	pb "go.viam.com/api/component/servo/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"braces.dev/errtrace"
	"go.viam.com/rdk/data"
)

type method int64

const (
	position method = iota
	doCommand
	getWorldPose
)

func (m method) String() string {
	switch m {
	case position:
		return "Position"
	case doCommand:
		return "DoCommand"
	case getWorldPose:
		return "GetWorldPose"
	}
	return "Unknown"
}

// newPositionCollector returns a collector to register a position method. If one is already registered
// with the same MethodMetadata it will panic.
func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	servo, err := assertServo(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		pos, err := servo.Position(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if data.IsNoCaptureToStoreError(err) {
				return res, errtrace.Wrap(err)
			}
			return res, errtrace.Wrap(data.NewFailedToReadError(params.ComponentName, position.String(), err))
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return errtrace.Wrap2(data.NewTabularCaptureResult(ts, pb.GetPositionResponse{
			PositionDeg: pos,
		}))
	})
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	servoResource, err := assertServo(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(servoResource, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newGetWorldPoseCollector returns a collector to capture the servo's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertServo(resource); err != nil {
		return nil, errtrace.Wrap(err)
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

func assertServo(resource interface{}) (Servo, error) {
	servo, ok := resource.(Servo)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return servo, nil
}
