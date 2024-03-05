package rtkutils

import (
	"context"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/spatialmath"
)

var errNilLocation = errors.New("nil gps location, check nmea message parsing")

// CachedGpsData allows the use of any MovementSensor chip via a DataReader.
type CachedGpsData struct {
	mu           sync.RWMutex
	uncachedData *GPSData

	err                movementsensor.LastError
	lastPosition       movementsensor.LastPosition
	lastCompassHeading movementsensor.LastCompassHeading
}

// NewCachedGpsData creates a new CachedGpsData object.
func NewCachedGpsData(uncachedData *GPSData) CachedGpsData {
	return CachedGpsData{
		err:                movementsensor.NewLastError(1, 1),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
		uncachedData:       uncachedData,
	}
}

// Position returns the position and altitide of the sensor, or an error.
func (g *CachedGpsData) Position(
	ctx context.Context, extra map[string]interface{},
) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastPosition := g.lastPosition.GetLastPosition()
	currentPosition := g.uncachedData.Location

	if currentPosition == nil {
		return lastPosition, 0, errNilLocation
	}

	// if current position is (0,0) we will return the last non-zero position
	if movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsZeroPosition(lastPosition) {
		return lastPosition, g.uncachedData.Alt, g.err.Get()
	}

	// updating the last known valid position if the current position is non-zero
	if !movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsPositionNaN(currentPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	return currentPosition, g.uncachedData.Alt, g.err.Get()
}

// Accuracy returns the accuracy map, hDOP, vDOP, Fixquality and compass heading error.
func (g *CachedGpsData) Accuracy(
	ctx context.Context, extra map[string]interface{},
) (*movementsensor.Accuracy, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	acc := movementsensor.Accuracy{
		AccuracyMap: map[string]float32{
			"hDOP": float32(g.uncachedData.HDOP),
			"vDOP": float32(g.uncachedData.VDOP),
		},
		Hdop:               float32(g.uncachedData.HDOP),
		Vdop:               float32(g.uncachedData.VDOP),
		NmeaFix:            int32(g.uncachedData.FixQuality),
		CompassDegreeError: float32(math.NaN()),
	}
	return &acc, g.err.Get()
}

// LinearVelocity returns the sensor's linear velocity. It requires having a compass heading, so we
// know which direction our speed is in. We assume all of this speed is horizontal, and not in
// gaining/losing altitude.
func (g *CachedGpsData) LinearVelocity(
	ctx context.Context, extra map[string]interface{},
) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if math.IsNaN(g.uncachedData.CompassHeading) {
		return r3.Vector{}, g.err.Get()
	}

	headingInRadians := g.uncachedData.CompassHeading * (math.Pi / 180)
	xVelocity := g.uncachedData.Speed * math.Sin(headingInRadians)
	yVelocity := g.uncachedData.Speed * math.Cos(headingInRadians)

	return r3.Vector{X: xVelocity, Y: yVelocity, Z: 0}, g.err.Get()
}

// LinearAcceleration returns the sensor's linear acceleration.
func (g *CachedGpsData) LinearAcceleration(
	ctx context.Context, extra map[string]interface{},
) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

// AngularVelocity returns the sensor's angular velocity.
func (g *CachedGpsData) AngularVelocity(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// Orientation returns the sensor's orientation.
func (g *CachedGpsData) Orientation(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.Orientation, error) {
	return nil, movementsensor.ErrMethodUnimplementedOrientation
}

// CompassHeading returns the heading, from the range 0->360.
func (g *CachedGpsData) CompassHeading(
	ctx context.Context, extra map[string]interface{},
) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastHeading := g.lastCompassHeading.GetLastCompassHeading()
	currentHeading := g.uncachedData.CompassHeading

	if !math.IsNaN(lastHeading) && math.IsNaN(currentHeading) {
		return lastHeading, nil
	}

	if !math.IsNaN(currentHeading) && currentHeading != lastHeading {
		g.lastCompassHeading.SetLastCompassHeading(currentHeading)
	}

	return currentHeading, nil
}

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *CachedGpsData) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.uncachedData.FixQuality, nil
}

// ReadSatsInView returns the number of satellites in view.
func (g *CachedGpsData) ReadSatsInView(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.uncachedData.SatsInView, nil
}

// Properties returns what movement sensor capabilities we have.
func (g *CachedGpsData) Properties(
	ctx context.Context, extra map[string]interface{},
) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
		CompassHeadingSupported: true,
	}, nil
}
