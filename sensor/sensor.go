// Package sensor defines an abstract sensing device that can provide measurement readings.
package sensor

import (
	"context"
)

// A Sensor represents a general purpose sensors that can give arbitrary readings
// of some thing that it is sensing.
type Sensor interface {
	// Readings return data specific to the type of sensor and can be of any type.
	Readings(ctx context.Context) ([]interface{}, error)

	// Desc returns a description of this sensor.
	Desc() Description

	// Reconfigure replaces this sensor with the given sensor.
	Reconfigure(newSensor Sensor)
}

// Type specifies the type of sensor.
type Type string

// Description describes information about the device.
type Description struct {
	Type Type

	// Path is some universal descriptor of how to find the device.
	Path string
}
