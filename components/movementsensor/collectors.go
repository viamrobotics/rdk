package movementsensor

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/anypb"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"
	"go.viam.com/rdk/data"
)

// TODO: add tests for this file.
type method int64

const (
	position method = iota
	linearVelocity
	angularVelocity
	compassHeading
	linearAcceleration
	orientation
)

func (m method) String() string {
	switch m {
	case position:
		return "Position"
	case linearVelocity:
		return "LinearVelocity"
	case angularVelocity:
		return "AngularVelocity"
	case compassHeading:
		return "CompassHeading"
	case linearAcceleration:
		return "LinearAcceleration"
	case orientation:
		return "Orientation"
	}
	return "Unknown"
}

func assertMovementSensor(resource interface{}) (MovementSensor, error) {
	ms, ok := resource.(MovementSensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return ms, nil
}

func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		vec, err := ms.LinearVelocity(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		return pb.GetLinearVelocityResponse{
			LinearVelocity: &v1.Vector3{
				X: vec.X,
				Y: vec.Y,
				Z: vec.Z,
			},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newLinearVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		pos, altitude, err := ms.Position(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, linearVelocity.String(), err)
		}
		return pb.GetPositionResponse{
			Coordinate: &v1.GeoPoint{
				Latitude:  pos.Lat(),
				Longitude: pos.Lng(),
			},
			AltitudeM: float32(altitude),
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newAngularVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		vel, err := ms.AngularVelocity(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, angularVelocity.String(), err)
		}
		return pb.GetAngularVelocityResponse{
			AngularVelocity: &v1.Vector3{
				X: vel.X,
				Y: vel.Y,
				Z: vel.Z,
			},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newCompassHeadingCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		heading, err := ms.CompassHeading(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, compassHeading.String(), err)
		}
		return pb.GetCompassHeadingResponse{
			Value: heading,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newLinearAccelerationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		accel, err := ms.LinearAcceleration(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, linearAcceleration.String(), err)
		}
		return pb.GetLinearAccelerationResponse{
			LinearAcceleration: &v1.Vector3{
				X: accel.X,
				Y: accel.Y,
				Z: accel.Z,
			},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

func newOrientationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		orient, err := ms.Orientation(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, orientation.String(), err)
		}
		axisAng := orient.AxisAngles()
		return pb.GetOrientationResponse{
			Orientation: &v1.Orientation{
				OX:    axisAng.RX,
				OY:    axisAng.RY,
				OZ:    axisAng.RZ,
				Theta: axisAng.Theta,
			},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}
