package gripper

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	doCommand method = iota
	getWorldPose
	isHoldingSomething
)

func (m method) String() string {
	switch m {
	case isHoldingSomething:
		return "IsHoldingSomething"
	case doCommand:
		return "DoCommand"
	case getWorldPose:
		return "GetWorldPose"
	}
	return "Unknown"
}

type gripperIsHolding struct {
	IsHoldingSomething bool                   `json:"is_holding_something"`
	Meta               map[string]interface{} `json:"meta"`
}

// newIsHoldingSomethingCollector returns a collector to register an is holding something method.
func newIsHoldingSomethingCollector(resource any, params data.CollectorParams) (data.Collector, error) {
	gripper, err := assertGripper(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		status, err := gripper.IsHoldingSomething(ctx, data.FromDMExtraMap)
		if err != nil {
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, isHoldingSomething.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, gripperIsHolding(status))
	})
	return data.NewCollector(cFunc, params)
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

// newGetWorldPoseCollector returns a collector to capture the gripper's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertGripper(resource); err != nil {
		return nil, err
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, err
	}
	return data.NewCollector(cFunc, params)
}

func assertGripper(resource interface{}) (Gripper, error) {
	gripper, ok := resource.(Gripper)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return gripper, nil
}
