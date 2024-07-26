//go:build linux

// Package gpsutils implements a GPS NMEA component. This file contains ways to read data from a
// PMTK device connected over the I2C bus.
package gpsutils

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	rdkutils "go.viam.com/rdk/utils"
)

// PmtkI2cDataReader implements the DataReader interface for a PMTK device by communicating with it
// over an I2C bus.
type PmtkI2cDataReader struct {
	data chan string

	workers rdkutils.StoppableWorkers
	logger  logging.Logger

	bus  buses.I2C
	addr byte
	baud int
}

// NewI2cDataReader constructs a new DataReader that gets its NMEA messages over an I2C bus.
func NewI2cDataReader(
	ctx context.Context,
	config I2CConfig,
	bus buses.I2C,
	logger logging.Logger,
) (DataReader, error) {
	if bus == nil {
		var err error
		bus, err = buses.NewI2cBus(config.I2CBus)
		if err != nil {
			return nil, fmt.Errorf("gps init: failed to find i2c bus %s: %w", config.I2CBus, err)
		}
	}
	addr := config.I2CAddr
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}
	baud := config.I2CBaudRate
	if baud == 0 {
		baud = 38400
		logger.Warn("using default baudrate: 38400")
	}

	data := make(chan string)
	reader := PmtkI2cDataReader{
		data:       data,
		logger:     logger,
		bus:        bus,
		addr:       byte(addr),
		baud:       baud,
	}

	if err := reader.initialize(ctx); err != nil {
		return nil, err
	}

	reader.workers = rdkutils.NewStoppableWorkers(reader.backgroundWorker)
	return &reader, nil
}

// initialize sends commands to the device to put it into a state where we can read data from it.
func (dr *PmtkI2cDataReader) initialize(ctx context.Context) error {
	handle, err := dr.bus.OpenHandle(dr.addr)
	if err != nil {
		dr.logger.CErrorf(ctx, "can't open gps i2c %s", err)
		return err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// Set the baud rate
	// TODO: does this actually do anything in the current context? The baud rate should be
	// governed by the clock line on the I2C bus, not on the device.
	baudcmd := fmt.Sprintf("PMTK251,%d", dr.baud)
	cmd251 := movementsensor.PMTKAddChk([]byte(baudcmd))
	// Output GLL, RMC, VTG, GGA, GSA, and GSV sentences, and nothing else, every position fix
	cmd314 := movementsensor.PMTKAddChk([]byte("PMTK314,1,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	// Ask for updates every 1000 ms (every second)
	cmd220 := movementsensor.PMTKAddChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		dr.logger.CDebug(ctx, "Failed to set baud rate")
		return err
	}
	err = handle.Write(ctx, cmd314)
	if err != nil {
		return err
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		return err
	}
	return nil
}

func (dr *PmtkI2cDataReader) readData(cancelCtx context.Context) ([]byte, error) {
	handle, err := dr.bus.OpenHandle(dr.addr)
	if err != nil {
		dr.logger.CErrorf(cancelCtx, "can't open gps i2c %s", err)
		return nil, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	buffer, err := handle.Read(cancelCtx, 1024)
	if err != nil {
		dr.logger.CErrorf(cancelCtx, "failed to read handle %s", err)
		return nil, err
	}

	return buffer, nil
}

// backgroundWorker should be run in a background coroutine. It reads data from the I2C bus and
// puts it into the channel of complete messages.
func (dr *PmtkI2cDataReader) backgroundWorker(cancelCtx context.Context) {
	defer close(dr.data)

	strBuf := ""
	for {
		select {
		case <-cancelCtx.Done():
			return
		default:
		}

		buffer, err := dr.readData(cancelCtx)
		if err != nil {
			dr.logger.CErrorf(cancelCtx, "failed to read data, retrying: %s", err)
			continue
		}

		for _, b := range buffer {
			if b == 0xFF {
				continue // This byte indicates that the chip did not have data to send us.
			}

			// Otherwise, the chip is trying to communicate with us. However, sometimes the
			// data has the most significant bit of the byte set, even though it should only
			// send ASCII (which never sets the most significant bit). So, to reduce checksum
			// errors, we mask out that bit.
			b &= 0x7F

			// PMTK uses CRLF line endings to terminate sentences, but just LF to blank data.
			// Since CR should never appear except at the end of our sentence, we use that to
			// determine sentence end. LF is merely ignored.
			if b == 0x0D { // 0x0D is the ASCII value for a carriage return
				if strBuf != "" {
					// Sometimes we miss "$" on the first message of the buffer. If the first
					// character we read is a "G", it's likely that this has occurred, and we
					// should add a "$" at the beginning.
					if strBuf[0] == 0x47 { // 0x47 is the ASCII value for "G"
						strBuf = "$" + strBuf
					}

					select {
					case <-cancelCtx.Done():
						return
					case dr.data <- strBuf:
						strBuf = ""
					}
				}
			} else if b != 0x0A { // Skip the newlines, as described earlier
				strBuf += string(b)
			}
		}
	}
}

// Messages returns the channel of complete NMEA sentences we have read off of the device. It's part
// of the DataReader interface.
func (dr *PmtkI2cDataReader) Messages() chan string {
	return dr.data
}

// Close is part of the DataReader interface. It shuts everything down.
func (dr *PmtkI2cDataReader) Close() error {
	dr.workers.Stop()
	return nil
}
