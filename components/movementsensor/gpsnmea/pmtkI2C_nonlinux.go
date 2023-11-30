//go:build !linux

// Package gpsnmea implements a GPS NMEA component. This file contains just a stub of a constructor
// for a Linux-only version of the component (using the I2C bus).
package gpsnmea

import (
	"context"
	"errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// NewPmtkI2CGPSNMEA implements a gps that communicates over i2c.
func NewPmtkI2CGPSNMEA(
	ctx context.Context,
	deps resource.Dependencies,
	name resource.Name,
	conf *Config,
	logger logging.Logger,
) (NmeaMovementSensor, error) {
	// The nil on this next line means "use a real I2C bus, because we're not going to pass in a
	// mock one."
	return nil, errors.New("all I2C components are only available on Linux")
}
