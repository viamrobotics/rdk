package input

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
	input, err := assertInput(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(input, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newGetWorldPoseCollector returns a collector to capture the input controller's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertInput(resource); err != nil {
		return nil, errtrace.Wrap(err)
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

func assertInput(resource interface{}) (Controller, error) {
	input, ok := resource.(Controller)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return input, nil
}
