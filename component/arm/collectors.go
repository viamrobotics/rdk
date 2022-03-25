package arm

import (
	"context"
	"os"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getEndPosition method = iota
	getJointPositions
)

func (m method) String() string {
	switch m {
	case getEndPosition:
		return "GetEndPosition"
	case getJointPositions:
		return "GetJointPositions"
	}
	return "Unknown"
}

func newGetEndPositionCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, queueSize int, logger golog.Logger) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := arm.GetEndPosition(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, getEndPosition.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, logger), nil
}

func newGetJointPositionsCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, queueSize int, logger golog.Logger) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := arm.GetJointPositions(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, getJointPositions.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, logger), nil
}

func assertArm(resource interface{}) (Arm, error) {
	arm, ok := resource.(Arm)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return arm, nil
}
