package input

import (
	"context"
	"errors"
	"time"

	v1 "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
)

type method int64

const (
	doCommand method = iota
	getWorldPose
)

func (m method) String() string {
	if m == doCommand {
		return "DoCommand"
	}
	if m == getWorldPose {
		return "GetWorldPose"
	}
	return "Unknown"
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	input, err := assertInput(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(input, params)
	return data.NewCollector(cFunc, params)
}

// worldPoseReading is a temporary struct used until a GetWorldPoseResponse proto is defined.
type worldPoseReading struct {
	Pose *v1.Pose `json:"pose"`
}

// newGetWorldPoseCollector returns a collector to capture the input controller's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertInput(resource); err != nil {
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

func assertInput(resource interface{}) (Controller, error) {
	input, ok := resource.(Controller)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return input, nil
}
