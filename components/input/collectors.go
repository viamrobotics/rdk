package input

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getControls method = iota
)

func (m method) String() string {
	switch m {
	case getControls:
		return "GetControls"
	}
	return "Unknown"
}

// Controls wraps the returned control input (specific Axis or Button) values.
type Controls struct {
	Controls []Control
}

func newGetControlsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	controller, err := assertController(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := controller.GetControls(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getControls.String(), err)
		}
		return Controls{Controls: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertController(resource interface{}) (Controller, error) {
	controller, ok := resource.(Controller)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return controller, nil
}
