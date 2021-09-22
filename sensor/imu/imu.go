// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"
	"go.viam.com/core/sensor"
)

const (
	Type         = "imu"
)

type IMU interface {
	sensor.Sensor
	// AngularVelocities returns rates of rotation across X, Y, Z axes measured in rad/s.
	AngularVelocities(ctx context.Context) ([3]float64, error)
	// Pitch returns current rotation around the X axis in rads.
	Pitch(ctx context.Context) (float64, error)
	// Roll returns current rotation around the Y axis in rads.
	Roll(ctx context.Context) (float64, error)
	// Yaw returns current rotation around the Z axis in rads.
	Yaw(ctx context.Context) (float64, error)
}