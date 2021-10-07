// Package gps defines the interfaces of a GPS device which provides lat/long
// measurements.
package gps

import (
	"context"

	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/core/sensor"
)

// The known GPS types.
const (
	Type = "gps"
)

// A GPS represents a GPS that can report lat/long measurements.
type GPS interface {
	sensor.Sensor
	Location(ctx context.Context) (*geo.Point, error)       // The current latitude and longitude
	Altitude(ctx context.Context) (float64, error)          // The current altitude in meters
	Speed(ctx context.Context) (float64, error)             // Current ground speed in kph
	Satellites(ctx context.Context) (int, int, error)       // Number of satellites used for fix, and total in view
	Accuracy(ctx context.Context) (float64, float64, error) // Horizontal and vertical position error
	Valid(ctx context.Context) (bool, error)                // Whether or not the GPS chip had a valid fix for the most recent dataset
}
