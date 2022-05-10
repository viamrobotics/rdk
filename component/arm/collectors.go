package arm

import (
	"context"

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

func newGetEndPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := arm.GetEndPosition(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getEndPosition.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func newGetJointPositionsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := arm.GetJointPositions(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getJointPositions.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertArm(resource interface{}) (Arm, error) {
	arm, ok := resource.(Arm)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return arm, nil
}
