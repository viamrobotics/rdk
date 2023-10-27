// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/adrianmo/go-nmea"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var errNilLocation = errors.New("nil gps location, check nmea message parsing")

// SerialNMEAMovementSensor allows the use of any MovementSensor chip that communicates over serial.
type SerialNMEAMovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	mu                      sync.RWMutex
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  logging.Logger
	data                    GPSData
	activeBackgroundWorkers sync.WaitGroup

	disableNmea        bool
	err                movementsensor.LastError
	lastPosition       movementsensor.LastPosition
	lastCompassHeading movementsensor.LastCompassHeading
	isClosed           bool

	dev                io.ReadWriteCloser
	path               string
	baudRate           uint
	correctionBaudRate uint
	correctionPath     string
}

// NewSerialGPSNMEA gps that communicates over serial.
func NewSerialGPSNMEA(ctx context.Context, name resource.Name, conf *Config, logger logging.Logger) (NmeaMovementSensor, error) {
	serialPath := conf.SerialConfig.SerialPath
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAMovementSensor expected non-empty string for %q", conf.SerialConfig.SerialPath)
	}

	baudRate := conf.SerialConfig.SerialBaudRate
	if baudRate == 0 {
		baudRate = 38400
		logger.Info("SerialNMEAMovementSensor: serial_baud_rate using default 38400")
	}

	disableNmea := conf.DisableNMEA
	if disableNmea {
		logger.Info("SerialNMEAMovementSensor: NMEA reading disabled")
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

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &SerialNMEAMovementSensor{
		Named:              name.AsNamed(),
		dev:                dev,
		cancelCtx:          cancelCtx,
		cancelFunc:         cancelFunc,
		logger:             logger,
		path:               serialPath,
		baudRate:           uint(baudRate),
		disableNmea:        disableNmea,
		err:                movementsensor.NewLastError(1, 1),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
	}

	if err := g.Start(ctx); err != nil {
		g.logger.Errorf("Did not create nmea gps with err %#v", err.Error())
	}

	return g, err
}

// Start begins reading nmea messages from module and updates gps data.
func (g *SerialNMEAMovementSensor) Start(ctx context.Context) error {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		r := bufio.NewReader(g.dev)
		for {
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			if !g.disableNmea && !g.isClosed {
				line, err := r.ReadString('\n')
				if err != nil {
					g.logger.Errorf("can't read gps serial %s", err)
					g.err.Set(err)
					return
				}
				// Update our struct's gps data in-place
				g.mu.Lock()
				err = g.data.ParseAndUpdate(line)
				g.mu.Unlock()
				if err != nil {
					g.logger.Warnf("can't parse nmea sentence: %#v", err)
				}
			}
		}
	})

	return g.err.Get()
}

// GetCorrectionInfo returns the serial path that takes in rtcm corrections and baudrate for reading.
func (g *SerialNMEAMovementSensor) GetCorrectionInfo() (string, uint) {
	return g.correctionPath, g.correctionBaudRate
}

//nolint
// Position position, altitide.
func (g *SerialNMEAMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	lastPosition := g.lastPosition.GetLastPosition()

	g.mu.RLock()
	defer g.mu.RUnlock()

	currentPosition := g.data.Location

	if currentPosition == nil {
		return lastPosition, 0, errNilLocation
	}

	// if current position is (0,0) we will return the last non zero position
	if g.lastPosition.IsZeroPosition(currentPosition) && !g.lastPosition.IsZeroPosition(lastPosition) {
		return lastPosition, g.data.Alt, g.err.Get()
	}

	// updating lastPosition if it is different from the current position
	if !g.lastPosition.ArePointsEqual(currentPosition, lastPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	// updating the last known valid position if the current position is non-zero
	if !g.lastPosition.IsZeroPosition(currentPosition) && !g.lastPosition.IsPositionNaN(currentPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	return currentPosition, g.data.Alt, g.err.Get()
}

// Accuracy returns the accuracy, hDOP and vDOP.
func (g *SerialNMEAMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]float32{"hDOP": float32(g.data.HDOP), "vDOP": float32(g.data.VDOP)}, nil
}

// LinearVelocity linear velocity.
func (g *SerialNMEAMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
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

// LinearAcceleration linear acceleration.
func (g *SerialNMEAMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

// AngularVelocity angularvelocity.
func (g *SerialNMEAMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// Orientation orientation.
func (g *SerialNMEAMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewOrientationVector(), movementsensor.ErrMethodUnimplementedOrientation
}

// CompassHeading 0->360.
func (g *SerialNMEAMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
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

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *SerialNMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.FixQuality, nil
}

// ReadSatsInView returns the number of satellites in view.
func (g *SerialNMEAMovementSensor) ReadSatsInView(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.SatsInView, nil
}

// Readings will use return all of the MovementSensor Readings.
func (g *SerialNMEAMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
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

// Properties what do I do!
func (g *SerialNMEAMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
		CompassHeadingSupported: true,
	}, nil
}

// Close shuts down the SerialNMEAMovementSensor.
func (g *SerialNMEAMovementSensor) Close(ctx context.Context) error {
	g.logger.Debug("Closing SerialNMEAMovementSensor")
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	g.mu.Lock()
	defer g.mu.Unlock()
	g.isClosed = true
	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			return err
		}
		g.dev = nil
		g.logger.Debug("SerialNMEAMovementSensor Closed")
	}
	return nil
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
