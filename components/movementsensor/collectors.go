package movementsensor

import (
	"context"
	"errors"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/protoutils"
)

type method int64

const (
	position method = iota
	linearVelocity
	angularVelocity
	compassHeading
	linearAcceleration
	orientation
	readings
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
	case readings:
		return "Readings"
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

// NewLinearVelocityCollector returns a collector to register a linear velocity method. If one is already registered
// with the same MethodMetadata it will panic.
func NewLinearVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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

// NewPositionCollector returns a collector to register a position method. If one is already registered
// with the same MethodMetadata it will panic.
func NewPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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

// NewAngularVelocityCollector returns a collector to register an angular velocity method. If one is already registered
// with the same MethodMetadata it will panic.
func NewAngularVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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

// NewCompassHeadingCollector returns a collector to register a compass heading method. If one is already registered
// with the same MethodMetadata it will panic.
func NewCompassHeadingCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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

// NewLinearAccelerationCollector returns a collector to register a linear acceleration method. If one is already registered
// with the same MethodMetadata it will panic.
func NewLinearAccelerationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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

// NewOrientationCollector returns a collector to register an orientation method. If one is already registered
// with the same MethodMetadata it will panic.
func NewOrientationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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

// NewReadingsCollector returns a collector to register a readings method. If one is already registered
// with the same MethodMetadata it will panic.
func NewReadingsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		values, err := ms.Readings(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}
		readings, err := protoutils.ReadingGoToProto(values)
		if err != nil {
			return nil, err
		}
		return v1.GetReadingsResponse{
			Readings: readings,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}
