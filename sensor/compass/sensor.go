package compass

import "github.com/viamrobotics/robotcore/sensor"

type Device interface {
	sensor.Device
	Heading() (float64, error)
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
