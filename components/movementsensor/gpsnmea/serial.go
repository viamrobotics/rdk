// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

import (
	"context"
	"fmt"
	"math"
	"sync"

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

	err                movementsensor.LastError
	lastPosition       movementsensor.LastPosition
	lastCompassHeading movementsensor.LastCompassHeading
	isClosed           bool

	dev DataReader
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

// Start begins reading nmea messages from module and updates gps data.
func (g *SerialNMEAMovementSensor) Start(ctx context.Context) error {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()

		messages := g.dev.Messages()
		for {
			// First, check if we're supposed to shut down.
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			// Next, wait until either we're supposed to shut down or we have new data to process.
			select {
			case <-g.cancelCtx.Done():
				return
			case message := <-messages:
				// Update our struct's gps data in-place
				g.mu.Lock()
				err := g.data.ParseAndUpdate(message)
				g.mu.Unlock()
				if err != nil {
					g.logger.CWarnf(ctx, "can't parse nmea sentence: %#v", err)
					g.logger.Debug("Check: GPS requires clear sky view." +
						"Ensure the antenna is outdoors if signal is weak or unavailable indoors.")
				}
			}

			if g.isClosed { // There's no coming back from this. We're done.
				return
			}
		}
	})

	return g.err.Get()
}

// Position returns the position and altitide of the sensor, or an error.
func (g *SerialNMEAMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastPosition := g.lastPosition.GetLastPosition()
	currentPosition := g.data.Location

	if currentPosition == nil {
		return lastPosition, 0, errNilLocation
	}

	// if current position is (0,0) we will return the last non-zero position
	if movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsZeroPosition(lastPosition) {
		return lastPosition, g.data.Alt, g.err.Get()
	}

	// updating the last known valid position if the current position is non-zero
	if !movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsPositionNaN(currentPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	return currentPosition, g.data.Alt, g.err.Get()
}

// Accuracy returns the accuracy map, hDOP, vDOP, Fixquality and compass heading error.
func (g *SerialNMEAMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
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

// LinearVelocity returns the sensor's linear velocity. It requires having a compass heading, so we
// know which direction our speed is in. We assume all of this speed is horizontal, and not in
// gaining/losing altitude.
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

// LinearAcceleration returns the sensor's linear acceleration.
func (g *SerialNMEAMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

// AngularVelocity returns the sensor's angular velocity.
func (g *SerialNMEAMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// Orientation returns the sensor's orientation.
func (g *SerialNMEAMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewOrientationVector(), movementsensor.ErrMethodUnimplementedOrientation
}

// CompassHeading returns the heading, from the range 0->360.
func (g *SerialNMEAMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastHeading := g.lastCompassHeading.GetLastCompassHeading()
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

// Properties returns what movement sensor capabilities we have.
func (g *SerialNMEAMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
		CompassHeadingSupported: true,
	}, nil
}

// Close shuts down the SerialNMEAMovementSensor.
func (g *SerialNMEAMovementSensor) Close(ctx context.Context) error {
	g.logger.CDebug(ctx, "Closing SerialNMEAMovementSensor")
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
		g.logger.CDebug(ctx, "SerialNMEAMovementSensor Closed")
	}
	return nil
}
