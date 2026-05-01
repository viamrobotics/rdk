package button

import (


	"go.viam.com/rdk/data"
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
	button, err := assertButton(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(button, params)
	return data.NewCollector(cFunc, params)
}


// newGetWorldPoseCollector returns a collector to capture the button's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertButton(resource); err != nil {
		return nil, err
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, err
	}
	return data.NewCollector(cFunc, params)
}

func assertButton(resource interface{}) (Button, error) {
	button, ok := resource.(Button)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return button, nil
}
