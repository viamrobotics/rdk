package generic

import (
	"braces.dev/errtrace"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
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
	reso, err := assertResource(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(reso, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// newGetWorldPoseCollector returns a collector to capture the resource's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(res interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertResource(res); err != nil {
		return nil, errtrace.Wrap(err)
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

// Resource is the interface that must be implemented by all resources that want to use the DoCommand collector.
type Resource interface {
	resource.Resource
}

func assertResource(resource interface{}) (resource.Resource, error) {
	res, ok := resource.(Resource)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return res, nil
}
