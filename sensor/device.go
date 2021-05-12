// Package sensor defines an abstract sensing device that can provide measurement readings.
package sensor

import (
	"context"
)

// A Device represents a general purpose sensors that can give arbitrary readings
// of some thing that it is sensing.
type Device interface {
	Readings(ctx context.Context) ([]interface{}, error)
	Desc() DeviceDescription
}

// DeviceType specifies the type of sensor.
type DeviceType string

// DeviceDescription describes information about the device.
type DeviceDescription struct {
	Type DeviceType

	// Path is some universal descriptor of how to find the device.
	Path string
}
