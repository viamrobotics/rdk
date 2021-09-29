// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"

	"go.viam.com/core/sensor"
)

// The known IMU types.
const (
	Type = "imu"
)

// An IMU represents and IMU that can report AngularVelocity and Orientation measurements.
type IMU interface {
	sensor.Sensor
	// AngularVelocity returns rates of rotation across X, Y, Z axes measured in rad/s.
	// TODO: I don't really like that this is an array that requires you to know the order. Maybe a map with an enum of
	// keys? Some custom type?
	AngularVelocity(ctx context.Context) (map[string]float64, error)
	// Orientation returns pitch (x), roll (y), and yaw (z) in rads.
	Orientation(ctx context.Context) (map[string]float64, error)
}
