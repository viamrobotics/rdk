package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/core/spatialmath"

	"go.viam.com/core/component/imu"
)

// IMU is an injected IMU.
type IMU struct {
	imu.IMU
	AngularVelocityFunc func(ctx context.Context) (*spatialmath.AngularVelocity, error)
	OrientationFunc     func(ctx context.Context) (*spatialmath.EulerAngles, error)
	CloseFunc           func() error
}

// AngularVelocity calls the injected AngularVelocity or the real version.
func (i *IMU) AngularVelocity(ctx context.Context) (*spatialmath.AngularVelocity, error) {
	if i.AngularVelocityFunc == nil {
		return i.IMU.AngularVelocity(ctx)
	}
	return i.AngularVelocityFunc(ctx)
}

// Orientation calls the injected Orientation or the real version.
func (i *IMU) Orientation(ctx context.Context) (*spatialmath.EulerAngles, error) {
	if i.OrientationFunc == nil {
		return i.IMU.Orientation(ctx)
	}
	return i.OrientationFunc(ctx)
}

// Close calls the injected Close or the real version.
func (i *IMU) Close() error {
	if i.CloseFunc == nil {
		return utils.TryClose(i.IMU)
	}
	return i.CloseFunc()
}
