package lidar

import "image"

type Device interface {
	Start()
	Stop()
	Close()
	// assumes the device is in a fixed point for the duration
	// of the scan
	Scan() (Measurements, error)
	Range() int
	Bounds() (image.Point, error)
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
