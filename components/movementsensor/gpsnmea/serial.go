// Package gpsnmea implements an NMEA gps.
package gpsnmea

import (
	"context"

	"go.viam.com/rdk/components/movementsensor/rtkutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// NewSerialGPSNMEA creates a component that communicates over a serial port.
func NewSerialGPSNMEA(ctx context.Context, name resource.Name, conf *Config, logger logging.Logger) (NmeaMovementSensor, error) {
	dev, err := rtkutils.NewSerialDataReader(conf.SerialConfig, logger)
	if err != nil {
		return nil, err
	}

	return NewNmeaMovementSensor(ctx, name, dev, logger)
}
