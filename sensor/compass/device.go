package compass

import (
	"github.com/viamrobotics/robotcore/sensor"
	"github.com/viamrobotics/robotcore/utils"
)

type Device interface {
	sensor.Device
	Heading() (float64, error)
	StartCalibration() error
	StopCalibration() error
}

type RelativeDevice interface {
	Device
	Mark() error
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

func AverageHeading(device Device) (float64, error) {
	var headings []float64
	numReadings := 5
	for i := 0; i < numReadings; i++ {
		heading, err := device.Heading()
		if err != nil {
			return 0, err
		}
		headings = append(headings, heading)
	}
	return utils.AverageAngleDeg(headings...), nil
}
