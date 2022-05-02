package inject

import (
	"context"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/spatialmath"
)

// IMU is an injected IMU.
type IMU struct {
	imu.IMU
	DoFunc                  func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ReadAngularVelocityFunc func(ctx context.Context) (spatialmath.AngularVelocity, error)
	ReadOrientationFunc     func(ctx context.Context) (spatialmath.Orientation, error)
	ReadAccelerationFunc    func(ctx context.Context) (r3.Vector, error)
	ReadMagnetometerFunc    func(ctx context.Context) (r3.Vector, error)
	CloseFunc               func(ctx context.Context) error
}

// ReadAngularVelocity calls the injected ReadAngularVelocity or the real version.
func (i *IMU) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	if i.ReadAngularVelocityFunc == nil {
		return i.IMU.ReadAngularVelocity(ctx)
	}
	return i.ReadAngularVelocityFunc(ctx)
}

// ReadOrientation calls the injected Orientation or the real version.
func (i *IMU) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	if i.ReadOrientationFunc == nil {
		return i.IMU.ReadOrientation(ctx)
	}
	return i.ReadOrientationFunc(ctx)
}

// ReadAcceleration calls the injected Acceleration or the real version.
func (i *IMU) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	if i.ReadAccelerationFunc == nil {
		return i.IMU.ReadAcceleration(ctx)
	}
	return i.ReadAccelerationFunc(ctx)
}

// ReadMagnetometer calls the injected Magnetometer or the real version.
func (i *IMU) ReadMagnetometer(ctx context.Context) (r3.Vector, error) {
	if i.ReadMagnetometerFunc == nil {
		return i.IMU.ReadMagnetometer(ctx)
	}
	return i.ReadMagnetometerFunc(ctx)
}

// Close calls the injected Close or the real version.
func (i *IMU) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.IMU)
	}
	return i.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (i *IMU) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.DoFunc == nil {
		return i.IMU.Do(ctx, cmd)
	}
	return i.DoFunc(ctx, cmd)
}
