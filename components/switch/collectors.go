package toggleswitch

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
	sw, err := assertToggleSwitch(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(sw, params)
	return data.NewCollector(cFunc, params)
}

func assertToggleSwitch(resource interface{}) (Switch, error) {
	sw, ok := resource.(Switch)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return sw, nil
}
