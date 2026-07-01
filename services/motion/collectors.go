package motion

import (
	"braces.dev/errtrace"
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
	motion, err := assertMotion(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(motion, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

func assertMotion(resource interface{}) (Service, error) {
	motion, ok := resource.(Service)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return motion, nil
}
