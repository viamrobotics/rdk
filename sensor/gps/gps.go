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
	Location(ctx context.Context) (lat float64, long float64, err error)
	Altitude(ctx context.Context) (alt float64, err error)
	Speed(ctx context.Context) (kph float64, err error)
	Satellites(ctx context.Context) (active, total int, err error)
	Accuracy(ctx context.Context) (horizontal, vertical float64, err error)
	Valid(ctx context.Context) (valid bool, err error)
}
