package generic

import (
	"context"
	"errors"
	"time"

	v1 "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
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
	reso, err := assertResource(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(reso, params)
	return data.NewCollector(cFunc, params)
}

// worldPoseReading is a temporary struct used until a GetWorldPoseResponse proto is defined.
type worldPoseReading struct {
	Pose *v1.Pose `json:"pose"`
}

// newGetWorldPoseCollector returns a collector to capture the resource's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(res interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertResource(res); err != nil {
		return nil, err
	}
	if params.FrameSystem == nil {
		return nil, errors.New("frame system is required for GetWorldPose collector")
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var captureRes data.CaptureResult
		pose, err := params.FrameSystem.GetPose(ctx, params.ComponentName, referenceframe.World, nil, data.FromDMExtraMap)
		if err != nil {
			if data.IsNoCaptureToStoreError(err) {
				return captureRes, err
			}
			return captureRes, data.NewFailedToReadError(params.ComponentName, getWorldPose.String(), err)
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

// Resource is the interface that must be implemented by all resources that want to use the DoCommand collector.
type Resource interface {
	resource.Resource
}

func assertResource(resource interface{}) (resource.Resource, error) {
	res, ok := resource.(Resource)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return res, nil
}
