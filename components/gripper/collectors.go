package gripper

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
	getFrameSystemPose
)

func (m method) String() string {
	switch m {
	case doCommand:
		return "DoCommand"
	case getFrameSystemPose:
		return "GetFrameSystemPose"
	}
	return "Unknown"
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gripper, err := assertGripper(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(gripper, params)
	return data.NewCollector(cFunc, params)
}

// frameSystemPoseReading is a temporary struct used until a GetFrameSystemPoseResponse proto is defined.
type frameSystemPoseReading struct {
	Pose *v1.Pose `json:"pose"`
}

// newGetFrameSystemPoseCollector returns a collector to capture the gripper's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetFrameSystemPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertGripper(resource); err != nil {
		return nil, err
	}
	if params.FrameSystem == nil {
		return nil, errors.New("frame system is required for GetFrameSystemPose collector")
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		pose, err := params.FrameSystem.GetPose(ctx, params.ComponentName, referenceframe.World, nil, data.FromDMExtraMap)
		if err != nil {
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, getFrameSystemPose.String(), err)
		}
		p := pose.Pose()
		o := p.Orientation().OrientationVectorDegrees()
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, frameSystemPoseReading{
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

func assertGripper(resource interface{}) (Gripper, error) {
	gripper, ok := resource.(Gripper)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return gripper, nil
}
