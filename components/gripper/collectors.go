package gripper

import (
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

func assertGripper(resource interface{}) (Gripper, error) {
	gripper, ok := resource.(Gripper)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return gripper, nil
}
