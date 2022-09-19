package gripper

import (
	"context"

	"go.viam.com/rdk/data"
	"google.golang.org/protobuf/types/known/anypb"
)

type method int64

const (
	grab method = iota
)

func (m method) String() string {
	switch m {
	case grab:
		return "Grab"
	}
	return "Unknown"
}

// Grab wraps the returned grab success value.
type Grab struct {
	Grab bool
}

func newGrabCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gripper, err := assertGripper(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := gripper.Grab(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, grab.String(), err)
		}
		return Grab{Grab: v}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertGripper(resource interface{}) (Gripper, error) {
	gripper, ok := resource.(Gripper)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return gripper, nil
}
