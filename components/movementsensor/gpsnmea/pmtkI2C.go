//go:build linux

// Package gpsnmea implements a GPS NMEA component.
package gpsnmea

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// I2cDataReader implements the DataReader interface by communicating with a device over an I2C bus.
type I2cDataReader struct {
	data chan string

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	logger                  logging.Logger

	bus  buses.I2C
	addr byte
	baud int
}

// NewI2cDataReader constructs a new DataReader that gets its NMEA messages over an I2C bus.
func NewI2cDataReader(
	bus buses.I2C, addr byte, baud int, logger logging.Logger,
) (DataReader, error) {
	data := make(chan string)
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	reader := I2cDataReader{
		data:       data,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		bus:        bus,
		addr:       addr,
		baud:       baud,
	}

	if err := reader.initialize(); err != nil {
		return nil, err
	}

	reader.start()
	return &reader, nil
}

// initialize sends commands to the device to put it into a state where we can read data from it.
func (dr *I2cDataReader) initialize() error {
	handle, err := dr.bus.OpenHandle(dr.addr)
	if err != nil {
		dr.logger.CErrorf(dr.cancelCtx, "can't open gps i2c %s", err)
		return err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// Set the baud rate
	// TODO: does this actually do anything in the current context? The baud rate should be
	// governed by the clock line on the I2C bus, not on the device.
	baudcmd := fmt.Sprintf("PMTK251,%d", dr.baud)
	cmd251 := addChk([]byte(baudcmd))
	// Output GLL, RMC, VTG, GGA, GSA, and GSV sentences, and nothing else, every position fix
	cmd314 := addChk([]byte("PMTK314,1,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	// Ask for updates every 1000 ms (every second)
	cmd220 := addChk([]byte("PMTK220,1000"))

	err = handle.Write(dr.cancelCtx, cmd251)
	if err != nil {
		dr.logger.CDebug(dr.cancelCtx, "Failed to set baud rate")
		return err
	}
	err = handle.Write(dr.cancelCtx, cmd314)
	if err != nil {
		return err
	}
	err = handle.Write(dr.cancelCtx, cmd220)
	if err != nil {
		return err
	}
	return nil
}

// start spins up a background coroutine to read data from the I2C bus and put it into the channel
// of complete messages.
func (dr *I2cDataReader) start() {
	dr.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer dr.activeBackgroundWorkers.Done()

		strBuf := ""
		for {
			select {
			case <-dr.cancelCtx.Done():
				return
			default:
			}

			// Opening an i2c handle blocks the whole bus, so we open/close each loop so other
			// things also have a chance to use it
			handle, err := dr.bus.OpenHandle(dr.addr)
			if err != nil {
				dr.logger.CErrorf(dr.cancelCtx, "can't open gps i2c handle, aborting: %s", err)
				return
			}
			buffer, err := handle.Read(dr.cancelCtx, 1024)
			if err != nil {
				dr.logger.CErrorf(dr.cancelCtx, "failed to read handle, retrying: %s", err)
				continue
			}
			err = handle.Close()
			if err != nil {
				dr.logger.CErrorf(dr.cancelCtx, "failed to close handle, aborting: %s", err)
				return
			}

			for _, b := range buffer {
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

						dr.data <- strBuf
						strBuf = ""
					}
				} else if b != 0x0A && b < 0x7F { // only append valid (printable) bytes
					strBuf += string(b)
				}
			}
		}
	})
}

// Messages returns the channel of complete NMEA sentences we have read off of the device. It's part
// of the DataReader interface.
func (dr *I2cDataReader) Messages() chan string {
	return dr.data
}

// Close is part of the DataReader interface. It shuts everything down.
func (dr *I2cDataReader) Close() error {
	dr.cancelFunc()
	dr.activeBackgroundWorkers.Wait()
	return nil
}

// PmtkI2CNMEAMovementSensor allows the use of any MovementSensor chip that communicates over I2C using the PMTK protocol.
type PmtkI2CNMEAMovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	mu                      sync.RWMutex
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  logging.Logger
	data                    GPSData
	activeBackgroundWorkers sync.WaitGroup

	err                movementsensor.LastError
	lastPosition       movementsensor.LastPosition
	lastCompassHeading movementsensor.LastCompassHeading

	bus   buses.I2C
	addr  byte
	wbaud int
}

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
	return MakePmtkI2cGpsNmea(ctx, deps, name, conf, logger, nil)
}

// MakePmtkI2cGpsNmea is only split out for ease of testing: you can pass in your own mock I2C bus,
// or pass in nil to have it create a real one. It is public so it can also be called from within
// the gpsrtkpmtk package.
func MakePmtkI2cGpsNmea(
	ctx context.Context,
	deps resource.Dependencies,
	name resource.Name,
	conf *Config,
	logger logging.Logger,
	i2cBus buses.I2C,
) (NmeaMovementSensor, error) {
	if i2cBus == nil {
		var err error
		i2cBus, err = buses.NewI2cBus(conf.I2CConfig.I2CBus)
		if err != nil {
			return nil, fmt.Errorf("gps init: failed to find i2c bus %s: %w",
				conf.I2CConfig.I2CBus, err)
		}
	}
	addr := conf.I2CConfig.I2CAddr
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}
	if conf.I2CConfig.I2CBaudRate == 0 {
		conf.I2CConfig.I2CBaudRate = 38400
		logger.CWarn(ctx, "using default baudrate : 38400")
	}

	dev, err := NewI2cDataReader(i2cBus, byte(addr), conf.I2CConfig.I2CBaudRate, logger)
	if err != nil {
		return nil, err
	}

	return NewNmeaMovementSensor(ctx, name, dev, logger)
}

// Properties what can I do!
func (g *PmtkI2CNMEAMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
		CompassHeadingSupported: true,
	}, nil
}

// ReadFix returns quality.
func (g *PmtkI2CNMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.FixQuality, g.err.Get()
}

// ReadSatsInView return number of satellites in view.
func (g *PmtkI2CNMEAMovementSensor) ReadSatsInView(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.SatsInView, g.err.Get()
}

// Close shuts down the SerialNMEAMOVEMENTSENSOR.
func (g *PmtkI2CNMEAMovementSensor) Close(ctx context.Context) error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	return g.err.Get()
}

// PMTK checksums commands by XORing together each byte.
func addChk(data []byte) []byte {
	chk := checksum(data)
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}

func checksum(data []byte) byte {
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	return chk
}
