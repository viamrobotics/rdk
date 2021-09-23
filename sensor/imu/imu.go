// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"
	"go.viam.com/core/sensor"
)

const (
	Type = "imu"
)

type IMU interface {
	sensor.Sensor
	// AngularVelocities returns rates of rotation across X, Y, Z axes measured in rad/s.
	AngularVelocities(ctx context.Context) ([3]float64, error)
	// Orientation returns pitch (x), roll (y), and yaw (z) in rads.
	Orientation(ctx context.Context) ([3]float64, error)
}