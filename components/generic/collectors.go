package generic

import (
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
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
	reso, err := assertResource(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(reso, params)
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
