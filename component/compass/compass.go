// Package compass defines the interfaces of a Compass and a Relative Compass which
// provide yaw measurements.
package compass

import (
	"context"

	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/utils"
)

// The known compass types.
const (
	Type         = "compass"
	RelativeType = "relative_compass"
)

// A Compass represents a Compass that can report yaw measurements. It can also
// be requested to calibrate itself.
type Compass interface {
	sensor.Sensor
	Heading(ctx context.Context) (float64, error)
	StartCalibration(ctx context.Context) error
	StopCalibration(ctx context.Context) error
}

// A RelativeCompass is a Compass that has no reference point (like a magnetic north)
// and as such must be asked to "mark" its current position as zero degrees.
type RelativeCompass interface {
	Compass
	Mark(ctx context.Context) error
}

// MedianHeading returns the median of successive headings from the given compass.
func MedianHeading(ctx context.Context, device Compass) (float64, error) {
	var headings []float64
	numReadings := 5
	for i := 0; i < numReadings; i++ {
		heading, err := device.Heading(ctx)
		if err != nil {
			return 0, err
		}
		headings = append(headings, heading)
	}
	return utils.Median(headings...), nil
}
