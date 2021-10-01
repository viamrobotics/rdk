// Package gps defines the interfaces of a GPS device which provides lat/long
// measurements.
package gps

import (
	"context"

	"go.viam.com/core/sensor"
)

// The known GPS types.
const (
	Type = "gps"
)

// A GPS represents a GPS that can report lat/long measurements.
type GPS interface {
	sensor.Sensor
	Location(ctx context.Context) (lat float64, long float64, err error) // The current latitude and longitude
	Altitude(ctx context.Context) (alt float64, err error) // The current altitude in meters
	Speed(ctx context.Context) (kph float64, err error) // Current ground speed in kph
	Satellites(ctx context.Context) (active, total int, err error) // Number of satellites used for fix, and total in view
	Accuracy(ctx context.Context) (horizontal, vertical float64, err error) // Horizontal and vertical position error
	Valid(ctx context.Context) (valid bool, err error) // Whether or not the GPS chip had a valid fix for the most recent dataset
}
