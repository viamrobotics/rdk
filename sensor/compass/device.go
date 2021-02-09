package compass

import "github.com/viamrobotics/robotcore/sensor"

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
