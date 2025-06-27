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
	generic, err := assertGeneric(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(generic, params)
	return data.NewCollector(cFunc, params)
}

// Service is the interface that wraps the DoCommand method.
type Service interface {
	resource.Resource
}

func assertGeneric(resource interface{}) (Service, error) {
	generic, ok := resource.(Service)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return generic, nil
}
