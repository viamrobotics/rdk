// Package forcematrix defines the interface of a generic Force Matrix Sensor
// which provides a 2-dimensional array of integers that correlate to forces
// applied to the sensor.
package forcematrix

import (
	"context"

	"go.viam.com/core/sensor"
)

// The forcematrix sensor sub-type
const (
	Type = "forcematrix"
)

// A ForceMatrix represents a force sensor that outputs a 2-dimensional array
// with integers that correlate to the forces applied to the sensor.
type ForceMatrix interface {
	sensor.Sensor
	Matrix(ctx context.Context) ([][]int, error)
}
