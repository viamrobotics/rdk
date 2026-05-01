package motor

import (
	"context"
	"errors"
	"time"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/motor/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
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
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		v, err := motor.Position(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, position.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetPositionResponse{
			Position: v,
		})
	})
	return data.NewCollector(cFunc, params)
}

// newIsPoweredCollector returns a collector to register an is powered method. If one is already registered
// with the same MethodMetadata it will panic.
func newIsPoweredCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		v, powerPct, err := motor.IsPowered(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, isPowered.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.IsPoweredResponse{
			IsOn:     v,
			PowerPct: powerPct,
		})
	})
	return data.NewCollector(cFunc, params)
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	motor, err := assertMotor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(motor, params)
	return data.NewCollector(cFunc, params)
}

// worldPoseReading is a temporary struct used until a GetWorldPoseResponse proto is defined.
type worldPoseReading struct {
	Pose *v1.Pose `json:"pose"`
}

// newGetWorldPoseCollector returns a collector to capture the motor's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertMotor(resource); err != nil {
		return nil, err
	}
	if params.FrameSystem == nil {
		return nil, errors.New("frame system is required for GetWorldPose collector")
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		pose, err := params.FrameSystem.GetPose(ctx, params.ComponentName, referenceframe.World, nil, data.FromDMExtraMap)
		if err != nil {
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, getWorldPose.String(), err)
		}
		p := pose.Pose()
		o := p.Orientation().OrientationVectorDegrees()
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, worldPoseReading{
			Pose: &v1.Pose{
				X:     p.Point().X,
				Y:     p.Point().Y,
				Z:     p.Point().Z,
				OX:    o.OX,
				OY:    o.OY,
				OZ:    o.OZ,
				Theta: o.Theta,
			},
		})
	})
	return data.NewCollector(cFunc, params)
}

func assertMotor(resource interface{}) (Motor, error) {
	motor, ok := resource.(Motor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return motor, nil
}
