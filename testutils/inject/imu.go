package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/core/component/imu"
	"go.viam.com/core/sensor"
	"go.viam.com/core/spatialmath"
)

// IMU is an injected imu.
type IMU struct {
	imu.IMU
	ReadingsFunc        func(ctx context.Context) ([]interface{}, error)
	AngularVelocityFunc func(ctx context.Context) (spatialmath.AngularVelocity, error)
	OrientationFunc     func(ctx context.Context) (spatialmath.Orientation, error)
	CloseFunc           func() error
}

// Readings calls the injected Readings or the real version.
func (i *IMU) Readings(ctx context.Context) ([]interface{}, error) {
	if i.ReadingsFunc == nil {
		return i.IMU.Readings(ctx)
	}
	return i.ReadingsFunc(ctx)
}

// AngularVelocity calls the injected AngularVelocity or the real version.
func (i *IMU) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	if i.AngularVelocityFunc == nil {
		return i.IMU.AngularVelocity(ctx)
	}
	return i.AngularVelocityFunc(ctx)
}

// Orientation calls the injected Orientation or the real version.
func (i *IMU) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	if i.OrientationFunc == nil {
		return i.IMU.Orientation(ctx)
	}
	return i.OrientationFunc(ctx)
}

// Desc returns that this is an IMU.
func (i *IMU) Desc() sensor.Description {
	return sensor.Description{sensor.Type(imu.SubtypeName), ""}
}

// Close calls the injected Close or the real version.
func (i *IMU) Close() error {
	if i.CloseFunc == nil {
		return utils.TryClose(i.IMU)
	}
	return i.CloseFunc()
}
