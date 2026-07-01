package motor

import (
	"context"
	"time"

	pb "go.viam.com/api/component/motor/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"braces.dev/errtrace"
	"go.viam.com/rdk/data"
)

type method int64

const (
	position method = iota
	isPowered
	doCommand
	getWorldPose
)

func (m method) String() string {
	switch m {
	case position:
		return "Position"
	case isPowered:
		return "IsPowered"
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
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		v, err := motor.Position(ctx, data.FromDMExtraMap)
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
			Position: v,
		}))
	})
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newIsPoweredCollector returns a collector to register an is powered method. If one is already registered
// with the same MethodMetadata it will panic.
func newIsPoweredCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		v, powerPct, err := motor.IsPowered(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if data.IsNoCaptureToStoreError(err) {
				return res, errtrace.Wrap(err)
			}
			return res, errtrace.Wrap(data.NewFailedToReadError(params.ComponentName, isPowered.String(), err))
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return errtrace.Wrap2(data.NewTabularCaptureResult(ts, pb.IsPoweredResponse{
			IsOn:     v,
			PowerPct: powerPct,
		}))
	})
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(motor, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newGetWorldPoseCollector returns a collector to capture the motor's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertMotor(resource); err != nil {
		return nil, errtrace.Wrap(err)
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

func assertMotor(resource interface{}) (Motor, error) {
	motor, ok := resource.(Motor)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return motor, nil
}
