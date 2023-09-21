//go:build !no_cgo

package arm

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/anypb"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
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
		v, err := arm.EndPosition(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, endPosition.String(), err)
		}
		o := v.Orientation().OrientationVectorRadians()
		return pb.GetEndPositionResponse{
			Pose: &v1.Pose{
				X:     v.Point().X,
				Y:     v.Point().Y,
				Z:     v.Point().Z,
				OX:    o.OX,
				OY:    o.OY,
				OZ:    o.OZ,
				Theta: o.Theta,
			},
		}, nil

	})
	return data.NewCollector(cFunc, params)
}

func newJointPositionsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	arm, err := assertArm(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		v, err := arm.JointPositions(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, jointPositions.String(), err)
		}
		return pb.GetJointPositionsResponse{
			Positions: v,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertArm(resource interface{}) (Arm, error) {
	arm, ok := resource.(Arm)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return arm, nil
}
