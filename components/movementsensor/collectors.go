package movementsensor

import (
	"context"
	"errors"
	"time"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/spatialmath"
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

// newLinearVelocityCollector returns a collector to register a linear velocity method. If one is already registered
// with the same MethodMetadata it will panic.
//
//nolint:dupl
func newLinearVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		vec, err := ms.LinearVelocity(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetLinearVelocityResponse{
			LinearVelocity: &v1.Vector3{
				X: vec.X,
				Y: vec.Y,
				Z: vec.Z,
			},
		})
	})
	return data.NewCollector(cFunc, params)
}

// newPositionCollector returns a collector to register a position method. If one is already registered
// with the same MethodMetadata it will panic.
func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		pos, altitude, err := ms.Position(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, linearVelocity.String(), err)
		}
		var lat, lng float64
		if pos != nil {
			lat = pos.Lat()
			lng = pos.Lng()
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetPositionResponse{
			Coordinate: &v1.GeoPoint{
				Latitude:  lat,
				Longitude: lng,
			},
			AltitudeM: float32(altitude),
		})
	})
	return data.NewCollector(cFunc, params)
}

// newAngularVelocityCollector returns a collector to register an angular velocity method. If one is already registered
// with the same MethodMetadata it will panic.
//
//nolint:dupl
func newAngularVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		vel, err := ms.AngularVelocity(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, angularVelocity.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetAngularVelocityResponse{
			AngularVelocity: &v1.Vector3{
				X: vel.X,
				Y: vel.Y,
				Z: vel.Z,
			},
		})
	})
	return data.NewCollector(cFunc, params)
}

// newCompassHeadingCollector returns a collector to register a compass heading method. If one is already registered
// with the same MethodMetadata it will panic.
func newCompassHeadingCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		heading, err := ms.CompassHeading(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, compassHeading.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetCompassHeadingResponse{
			Value: heading,
		})
	})
	return data.NewCollector(cFunc, params)
}

// newLinearAccelerationCollector returns a collector to register a linear acceleration method. If one is already registered
// with the same MethodMetadata it will panic.
//
//nolint:dupl
func newLinearAccelerationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		accel, err := ms.LinearAcceleration(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, linearAcceleration.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetLinearAccelerationResponse{
			LinearAcceleration: &v1.Vector3{
				X: accel.X,
				Y: accel.Y,
				Z: accel.Z,
			},
		})
	})
	return data.NewCollector(cFunc, params)
}

// newOrientationCollector returns a collector to register an orientation method. If one is already registered
// with the same MethodMetadata it will panic.
func newOrientationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		orient, err := ms.Orientation(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, orientation.String(), err)
		}
		var orientVector *spatialmath.OrientationVectorDegrees
		if orient != nil {
			orientVector = orient.OrientationVectorDegrees()
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, pb.GetOrientationResponse{
			Orientation: &v1.Orientation{
				OX:    orientVector.OX,
				OY:    orientVector.OY,
				OZ:    orientVector.OZ,
				Theta: orientVector.Theta,
			},
		})
	})
	return data.NewCollector(cFunc, params)
}

// newReadingsCollector returns a collector to register a readings method. If one is already registered
// with the same MethodMetadata it will panic.
func newReadingsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ms, err := assertMovementSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		values, err := ms.Readings(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}

		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResultReadings(ts, values)
	})
	return data.NewCollector(cFunc, params)
}
