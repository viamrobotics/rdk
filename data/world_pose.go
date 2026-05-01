package data

import (
	"context"
	"errors"
	"time"

	v1 "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/referenceframe"
)

// worldPoseReading is a temporary struct until a GetWorldPoseResponse proto is defined.
type worldPoseReading struct {
	Pose *v1.Pose `json:"pose"`
}

// NewGetWorldPoseCaptureFunc returns a CaptureFunc that records a component's world-space pose via the frame system.
// Components should assert their specific type before calling this.
func NewGetWorldPoseCaptureFunc(params CollectorParams) (CaptureFunc, error) {
	if params.FrameSystem == nil {
		return nil, errors.New("frame system is required for GetWorldPose collector")
	}
	return func(ctx context.Context, _ map[string]*anypb.Any) (CaptureResult, error) {
		timeRequested := time.Now()
		var res CaptureResult
		pose, err := params.FrameSystem.GetPose(ctx, params.ComponentName, referenceframe.World, nil, FromDMExtraMap)
		if err != nil {
			if IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, NewFailedToReadError(params.ComponentName, "GetWorldPose", err)
		}
		p := pose.Pose()
		o := p.Orientation().OrientationVectorDegrees()
		ts := Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return NewTabularCaptureResult(ts, worldPoseReading{
			Pose: &v1.Pose{
				X: p.Point().X, Y: p.Point().Y, Z: p.Point().Z,
				OX: o.OX, OY: o.OY, OZ: o.OZ, Theta: o.Theta,
			},
		})
	}, nil
}
