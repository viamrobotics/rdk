// Package gpsnmea implements an NMEA gps.
package gpsnmea

import (
	"context"
	"fmt"

	"github.com/jacobsa/go-serial/serial"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// NewSerialGPSNMEA gps that communicates over serial.
func NewSerialGPSNMEA(ctx context.Context, name resource.Name, conf *Config, logger logging.Logger) (NmeaMovementSensor, error) {
	serialPath := conf.SerialConfig.SerialPath
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAMovementSensor expected non-empty string for %q", conf.SerialConfig.SerialPath)
	}

	baudRate := conf.SerialConfig.SerialBaudRate
	if baudRate == 0 {
		baudRate = 38400
		logger.CInfo(ctx, "SerialNMEAMovementSensor: serial_baud_rate using default 38400")
	}

	options := serial.OpenOptions{
		PortName:        serialPath,
		BaudRate:        uint(baudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	dev, err := NewSerialDataReader(options, logger)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &SerialNMEAMovementSensor{
		Named:              name.AsNamed(),
		dev:                dev,
		cancelCtx:          cancelCtx,
		cancelFunc:         cancelFunc,
		logger:             logger,
		err:                movementsensor.NewLastError(1, 1),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
	}

	if err := g.Start(ctx); err != nil {
		g.logger.CErrorf(ctx, "Did not create nmea gps with err %#v", err.Error())
	}

	return g, err
}
