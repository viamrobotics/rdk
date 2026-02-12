package base

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
	base, err := assertBase(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(base, params)
	return data.NewCollector(cFunc, params)
}

func assertBase(resource interface{}) (Base, error) {
	base, ok := resource.(Base)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return base, nil
}
