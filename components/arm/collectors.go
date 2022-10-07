package arm

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	endPosition method = iota
	jointPositions
)

func (m method) String() string {
	switch m {
	case endPosition:
		return "EndPosition"
	case jointPositions:
		return "JointPositions"
	}
	return "Unknown"
}

func newEndPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := arm.EndPosition(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, endPosition.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func newJointPositionsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := arm.JointPositions(ctx, nil)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, jointPositions.String(), err)
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
