package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/spatialmath"
)

// IMU is an injected IMU.
type IMU struct {
	imu.IMU
	ReadAngularVelocityFunc func(ctx context.Context) (spatialmath.AngularVelocity, error)
	OrientationFunc         func(ctx context.Context) (spatialmath.Orientation, error)
	ReadingsFunc            func(ctx context.Context) ([]interface{}, error)
	CloseFunc               func(ctx context.Context) error
}

// ReadAngularVelocity calls the injected ReadAngularVelocity or the real version.
func (i *IMU) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	if i.ReadAngularVelocityFunc == nil {
		return i.IMU.ReadAngularVelocity(ctx)
	}
	return i.ReadAngularVelocityFunc(ctx)
}

// Orientation calls the injected Orientation or the real version.
func (i *IMU) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	if i.OrientationFunc == nil {
		return i.IMU.Orientation(ctx)
	}
	return i.OrientationFunc(ctx)
}

// Readings calls the injected Readings or the real version.
func (i *IMU) Readings(ctx context.Context) ([]interface{}, error) {
	if i.ReadingsFunc == nil {
		return i.IMU.Readings(ctx)
	}
	return i.ReadingsFunc(ctx)
}

// Close calls the injected Close or the real version.
func (i *IMU) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.IMU)
	}
	return i.CloseFunc(ctx)
}
