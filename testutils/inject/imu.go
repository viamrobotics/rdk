package inject

import (
	"context"

	"go.viam.com/core/sensor/imu"
	"go.viam.com/utils"
)

type IMU struct {
	imu.IMU
	ReadingsFunc          func(ctx context.Context) ([]interface{}, error)
	AngularVelocitiesFunc func(ctx context.Context) ([]float64, error)
	OrientationFunc       func(ctx context.Context) ([]float64, error)
	CloseFunc             func() error
}

// Readings calls the injected Readings or the real version.
func (imuInst *IMU) Readings(ctx context.Context) ([]interface{}, error) {
	if imuInst.ReadingsFunc == nil {
		return imuInst.IMU.Readings(ctx)
	}
	return imuInst.ReadingsFunc(ctx)
}

// AngularVelocities calls the injected AngularVelocities or the real version.
func (imuInst *IMU) AngularVelocity(ctx context.Context) ([]float64, error) {
	if imuInst.AngularVelocitiesFunc == nil {
		return imuInst.IMU.AngularVelocity(ctx)
	}
	return imuInst.AngularVelocitiesFunc(ctx)
}

// Orientation calls the injected Orientation or the real version.
func (imuInst *IMU) Orientation(ctx context.Context) ([]float64, error) {
	if imuInst.OrientationFunc == nil {
		return imuInst.IMU.Orientation(ctx)
	}
	return imuInst.OrientationFunc(ctx)
}

// Close calls the injected Close or the real version.
func (imuInst *IMU) Close() error {
	if imuInst.CloseFunc == nil {
		return utils.TryClose(imuInst.IMU)
	}
	return imuInst.CloseFunc()
}
