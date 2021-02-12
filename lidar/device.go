package lidar

import "image"

type Device interface {
	Start()
	Stop()
	Close() error
	// assumes the device is in a fixed point for the duration
	// of the scan
	Scan(options ScanOptions) (Measurements, error)
	Range() int
	Bounds() (image.Point, error)
	AngularResolution() float64
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

type ScanOptions struct {
	Count    int
	NoFilter bool
}
