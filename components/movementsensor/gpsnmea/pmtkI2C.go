//go:build linux

// Package gpsnmea implements a GPS NMEA component.
package gpsnmea

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

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

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &PmtkI2CNMEAMovementSensor{
		Named:      name.AsNamed(),
		bus:        i2cBus,
		addr:       byte(addr),
		wbaud:      conf.I2CConfig.I2CBaudRate,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		// Overloaded boards can have flaky I2C busses. Only report errors if at least 5 of the
		// last 10 attempts have failed.
		err:                movementsensor.NewLastError(10, 5),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
	}

	if err := g.Start(ctx); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

// Start begins reading nmea messages from module and updates gps data.
func (g *PmtkI2CNMEAMovementSensor) Start(ctx context.Context) error {
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.CErrorf(ctx, "can't open gps i2c %s", err)
		return err
	}
	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	baudcmd := fmt.Sprintf("PMTK251,%d", g.wbaud)
	cmd251 := addChk([]byte(baudcmd))
	cmd314 := addChk([]byte("PMTK314,1,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		g.logger.CDebug(ctx, "Failed to set baud rate")
	}
	err = handle.Write(ctx, cmd314)
	if err != nil {
		g.logger.CErrorf(ctx, "i2c handle write failed %s", err)
		return err
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.CErrorf(ctx, "i2c handle write failed %s", err)
		return err
	}
	err = handle.Close()
	if err != nil {
		g.logger.CErrorf(ctx, "failed to close handle: %s", err)
		return err
	}

	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		strBuf := ""
		for {
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			// Opening an i2c handle blocks the whole bus, so we open/close each loop so other things also have a chance to use it
			handle, err := g.bus.OpenHandle(g.addr)
			// Record the error value no matter what. If it's nil, this will help suppress
			// ephemeral errors later.
			g.err.Set(err)
			if err != nil {
				g.logger.CErrorf(ctx, "can't open gps i2c handle: %s", err)
				return
			}
			buffer, err := handle.Read(ctx, 1024)
			g.err.Set(err)
			hErr := handle.Close()
			g.err.Set(hErr)
			if hErr != nil {
				g.logger.CErrorf(ctx, "failed to close handle: %s", hErr)
				return
			}
			if err != nil {
				g.logger.CError(ctx, err)
				continue
			}
			for _, b := range buffer {
				// PMTK uses CRLF line endings to terminate sentences, but just LF to blank data.
				// Since CR should never appear except at the end of our sentence, we use that to determine sentence end.
				// LF is merely ignored.
				if b == 0x0D {
					if strBuf != "" {
						// sometimes we miss "$" on the first message of the buffer. Here we are adding the missing
						// "$" to a valid nmea string.
						if strBuf[0] == 0x47 { // 0x47 is the ASCII value
							strBuf = "$" + strBuf
						}

						g.mu.Lock()
						err = g.data.ParseAndUpdate(strBuf)
						g.mu.Unlock()
						if err != nil {
							g.logger.CDebugf(ctx, "can't parse nmea sentence: %s, %v", strBuf, err)
							g.logger.Debug("Check: GPS requires clear sky view." +
								" Ensure the antenna is outdoors if signal is weak or unavailable indoors.")
						}
					}
					strBuf = ""
				} else if b != 0x0A && b < 0x7F { // adds only valid bytes
					strBuf += string(b)
				}
			}
		}
	})

	return g.err.Get()
}

// GetBusAddr returns the bus and address that takes in rtcm corrections.
func (g *PmtkI2CNMEAMovementSensor) GetBusAddr() (buses.I2C, byte) {
	return g.bus, g.addr
}

// Position returns the current geographic location of the MovementSensor.
func (g *PmtkI2CNMEAMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	lastPosition := g.lastPosition.GetLastPosition()

	g.mu.RLock()
	defer g.mu.RUnlock()

	currentPosition := g.data.Location

	if currentPosition == nil {
		return lastPosition, 0, errNilLocation
	}

	// if current position is (0,0) we will return the last non zero position
	if movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsZeroPosition(lastPosition) {
		return lastPosition, g.data.Alt, g.err.Get()
	}

	// updating lastPosition if it is different from the current position
	if !movementsensor.ArePointsEqual(currentPosition, lastPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	// updating the last known valid position if the current position is non-zero
	if !movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsPositionNaN(currentPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	return currentPosition, g.data.Alt, g.err.Get()
}

// Accuracy returns the accuracy, hDOP and vDOP.
func (g *PmtkI2CNMEAMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	acc := movementsensor.Accuracy{
		AccuracyMap:        map[string]float32{"hDOP": float32(g.data.HDOP), "vDOP": float32(g.data.VDOP)},
		Hdop:               float32(g.data.HDOP),
		Vdop:               float32(g.data.VDOP),
		NmeaFix:            int32(g.data.FixQuality),
		CompassDegreeError: float32(math.NaN()),
	}
	return &acc, g.err.Get()
}

// LinearVelocity returns the current speed of the MovementSensor.
func (g *PmtkI2CNMEAMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if math.IsNaN(g.data.CompassHeading) {
		return r3.Vector{}, g.err.Get()
	}

	headingInRadians := g.data.CompassHeading * (math.Pi / 180)
	xVelocity := g.data.Speed * math.Sin(headingInRadians)
	yVelocity := g.data.Speed * math.Cos(headingInRadians)
	return r3.Vector{X: xVelocity, Y: yVelocity, Z: 0}, g.err.Get()
}

// LinearAcceleration returns the current linear acceleration of the MovementSensor.
func (g *PmtkI2CNMEAMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

// AngularVelocity not supported.
func (g *PmtkI2CNMEAMovementSensor) AngularVelocity(
	ctx context.Context,
	extra map[string]interface{},
) (spatialmath.AngularVelocity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// CompassHeading returns the compass heading in degree (0->360).
func (g *PmtkI2CNMEAMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	lastHeading := g.lastCompassHeading.GetLastCompassHeading()

	g.mu.RLock()
	defer g.mu.RUnlock()

	currentHeading := g.data.CompassHeading

	if !math.IsNaN(lastHeading) && math.IsNaN(currentHeading) {
		return lastHeading, nil
	}

	if !math.IsNaN(currentHeading) && currentHeading != lastHeading {
		g.lastCompassHeading.SetLastCompassHeading(currentHeading)
	}

	return currentHeading, nil
}

// Orientation not supporter.
func (g *PmtkI2CNMEAMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return nil, movementsensor.ErrMethodUnimplementedOrientation
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

// Readings will use return all of the MovementSensor Readings.
func (g *PmtkI2CNMEAMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, g, extra)
	if err != nil {
		return nil, err
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}
	satsInView, err := g.ReadSatsInView(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix
	readings["satellites_in_view"] = satsInView

	return readings, nil
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
