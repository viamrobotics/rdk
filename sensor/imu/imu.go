// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"

	"go.viam.com/core/spatialmath"

	"go.viam.com/core/sensor"
)

// The known IMU types.
const (
	Type = "imu"
)

// AngularVelocity contains angular velocity in rads/s across x/y/z axes.
type AngularVelocity struct {
	x float64
	y float64
	z float64
}

// An IMU represents a sensor that can report AngularVelocity and Orientation measurements.
type IMU interface {
	sensor.Sensor
	AngularVelocity(ctx context.Context) (AngularVelocity, error)
	Orientation(ctx context.Context) (spatialmath.Orientation, error)
}
