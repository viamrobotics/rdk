package gripper

import (
	"braces.dev/errtrace"
	"go.viam.com/rdk/data"
)

type method int64

const (
	doCommand method = iota
	getWorldPose
)

func (m method) String() string {
	switch m {
	case doCommand:
		return "DoCommand"
	case getWorldPose:
		return "GetWorldPose"
	}
	return "Unknown"
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gripper, err := assertGripper(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(gripper, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newGetWorldPoseCollector returns a collector to capture the gripper's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertGripper(resource); err != nil {
		return nil, errtrace.Wrap(err)
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

func assertGripper(resource interface{}) (Gripper, error) {
	gripper, ok := resource.(Gripper)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return gripper, nil
}
