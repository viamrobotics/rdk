package discovery

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
	discovery, err := assertDiscovery(resource)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	cFunc := data.NewDoCommandCaptureFunc(discovery, params)
	return errtrace.Wrap2(data.NewCollector(cFunc, params))
}

func assertDiscovery(resource interface{}) (Service, error) {
	discovery, ok := resource.(Service)
	if !ok {
		return nil, errtrace.Wrap(data.InvalidInterfaceErr(API))
	}
	return discovery, nil
}
