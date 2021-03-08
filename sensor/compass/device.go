package compass

import (
	"context"

	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/utils"
)

type Device interface {
	sensor.Device
	Heading(ctx context.Context) (float64, error)
	StartCalibration(ctx context.Context) error
	StopCalibration(ctx context.Context) error
}

type RelativeDevice interface {
	Device
	Mark(ctx context.Context) error
}

type DeviceType string

const (
	DeviceTypeUnknown = "unknown"
	DeviceTypeFake    = "fake"
)

type DeviceDescription struct {
	Type DeviceType
	Path string
}

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
