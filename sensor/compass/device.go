// Package compass defines the interfaces of a Compass and a Relative Compass which
// provide yaw measurements.
package compass

import (
	"context"

	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/utils"
)

// The known compass types.
const (
	DeviceType         = "compass"
	RelativeDeviceType = "relative_compass"
)

// A Device represents a Compass that can report yaw measurements. It can also
// be requested to calibrate itself.
type Device interface {
	sensor.Device
	Heading(ctx context.Context) (float64, error)
	StartCalibration(ctx context.Context) error
	StopCalibration(ctx context.Context) error
}

// A RelativeDevice is a Compass that has no reference point (like a magnetic north)
// and as such must be asked to "mark" its current position as zero degrees.
type RelativeDevice interface {
	Device
	Mark(ctx context.Context) error
}

// MedianHeading returns the median of successive headings from the given compass.
func MedianHeading(ctx context.Context, device Device) (float64, error) {
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
