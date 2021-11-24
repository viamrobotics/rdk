// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"

	"go.viam.com/core/spatialmath"

	"go.viam.com/core/sensor"
)

// Type is the identifier of an IMU.
const Type = "imu"

// An IMU represents a sensor that can report AngularVelocity and Orientation measurements.
type IMU interface {
	sensor.Sensor
	AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error)
	Orientation(ctx context.Context) (spatialmath.Orientation, error)
}
