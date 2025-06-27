package button

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
	button, err := assertButton(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(button, params)
	return data.NewCollector(cFunc, params)
}

func assertButton(resource interface{}) (Button, error) {
	button, ok := resource.(Button)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return button, nil
}
