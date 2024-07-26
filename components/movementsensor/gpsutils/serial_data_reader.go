// Package gpsutils contains code shared between multiple GPS implementations. This file is about
// how to interact with a PMTK device (a device which gets data from GPS satellites) connected by a
// serial port.
package gpsutils

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/jacobsa/go-serial/serial"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// SerialDataReader implements the DataReader interface (defined in component.go) by interacting
// with the device over a serial port.
type SerialDataReader struct {
	dev     io.ReadWriteCloser
	data    chan string
	workers utils.StoppableWorkers
	logger  logging.Logger
}

// NewSerialDataReader constructs a new DataReader that gets its NMEA messages over a serial port.
func NewSerialDataReader(config *SerialConfig, logger logging.Logger) (DataReader, error) {
	serialPath := config.SerialPath
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAMovementSensor expected non-empty string for %q", config.SerialPath)
	}

	baudRate := config.SerialBaudRate
	if baudRate == 0 {
		baudRate = 38400
		logger.Info("SerialNMEAMovementSensor: serial_baud_rate using default 38400")
	}

	options := serial.OpenOptions{
		PortName:        serialPath,
		BaudRate:        uint(baudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	dev, err := serial.Open(options)
	if err != nil {
		return nil, err
	}

	data := make(chan string)

	reader := SerialDataReader{
		dev:    dev,
		data:   data,
		logger: logger,
	}
	reader.workers = utils.NewStoppableWorkers(reader.backgroundWorker)

	return &reader, nil
}

func (dr *SerialDataReader) backgroundWorker(cancelCtx context.Context) {
	defer close(dr.data)

	r := bufio.NewReader(dr.dev)
	for {
		// Even if r.ReadString(), below, always returns errors and we never get to the bottom of
		// the loop, make sure we can still exit when we're supposed to.
		select {
		case <-cancelCtx.Done():
			return
		default:
		}

		line, err := r.ReadString('\n')
		if err != nil {
			dr.logger.CErrorf(cancelCtx, "can't read gps serial %s", err)
			continue // The line has bogus data; don't put it in the channel.
		}

		select {
		case <-cancelCtx.Done():
			return
		case dr.data <- line:
		}
	}
}

// Messages returns the channel of complete NMEA sentences we have read off of the device. It's part
// of the DataReader interface.
func (dr *SerialDataReader) Messages() chan string {
	return dr.data
}

// Close is part of the DataReader interface. It shuts everything down, including our connection to
// the serial port.
func (dr *SerialDataReader) Close() error {
	dr.workers.Stop()
	return dr.dev.Close()
}
