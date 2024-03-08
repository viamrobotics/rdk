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
	"go.viam.com/rdk/utils"
)

var errNilLocation = errors.New("nil gps location, check nmea message parsing")

const earthRadiusKm = 6371 // Earth's radius in kilometers

// CachedData allows the use of any MovementSensor chip via a DataReader.
type CachedData struct {
	mu       sync.RWMutex
	nmeaData NmeaParser

	err                movementsensor.LastError
	lastPosition       movementsensor.LastPosition
	lastCompassHeading movementsensor.LastCompassHeading
}

// NewCachedData creates a new CachedData object.
func NewCachedData() CachedData {
	return CachedData{
		err:                movementsensor.NewLastError(1, 1),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
	}
}

// ParseAndUpdate passes the provided message into the inner NmeaParser object, which parses the
// NMEA message and updates its state to match.
func (g *CachedData) ParseAndUpdate(line string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.nmeaData.ParseAndUpdate(line)
}

// Position returns the position and altitide of the sensor, or an error.
func (g *CachedData) Position(
	ctx context.Context, extra map[string]interface{},
) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastPosition := g.lastPosition.GetLastPosition()
	currentPosition := g.nmeaData.Location

	if currentPosition == nil {
		return lastPosition, 0, errNilLocation
	}

	// if current position is (0,0) we will return the last non-zero position
	if movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsZeroPosition(lastPosition) {
		return lastPosition, g.nmeaData.Alt, g.err.Get()
	}

	// updating the last known valid position if the current position is non-zero
	if !movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsPositionNaN(currentPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	return currentPosition, g.nmeaData.Alt, g.err.Get()
}

// Accuracy returns the accuracy map, hDOP, vDOP, Fixquality and compass heading error.
func (g *CachedData) Accuracy(
	ctx context.Context, extra map[string]interface{},
) (*movementsensor.Accuracy, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	compassDegreeError := g.calculateCompassDegreeError()

	acc := movementsensor.Accuracy{
		AccuracyMap: map[string]float32{
			"hDOP": float32(g.nmeaData.HDOP),
			"vDOP": float32(g.nmeaData.VDOP),
		},
		Hdop:               float32(g.nmeaData.HDOP),
		Vdop:               float32(g.nmeaData.VDOP),
		NmeaFix:            int32(g.nmeaData.FixQuality),
		CompassDegreeError: float32(compassDegreeError),
	}
	return &acc, g.err.Get()
}

// LinearVelocity returns the sensor's linear velocity. It requires having a compass heading, so we
// know which direction our speed is in. We assume all of this speed is horizontal, and not in
// gaining/losing altitude.
func (g *CachedData) LinearVelocity(
	ctx context.Context, extra map[string]interface{},
) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if math.IsNaN(g.nmeaData.CompassHeading) {
		return r3.Vector{}, g.err.Get()
	}

	headingInRadians := g.nmeaData.CompassHeading * (math.Pi / 180)
	xVelocity := g.nmeaData.Speed * math.Sin(headingInRadians)
	yVelocity := g.nmeaData.Speed * math.Cos(headingInRadians)

	return r3.Vector{X: xVelocity, Y: yVelocity, Z: 0}, g.err.Get()
}

// LinearAcceleration returns the sensor's linear acceleration.
func (g *CachedData) LinearAcceleration(
	ctx context.Context, extra map[string]interface{},
) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

// AngularVelocity returns the sensor's angular velocity.
func (g *CachedData) AngularVelocity(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// Orientation returns the sensor's orientation.
func (g *CachedData) Orientation(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.Orientation, error) {
	return nil, movementsensor.ErrMethodUnimplementedOrientation
}

// CompassHeading returns the heading, from the range 0->360.
func (g *CachedData) CompassHeading(
	ctx context.Context, extra map[string]interface{},
) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastHeading := g.lastCompassHeading.GetLastCompassHeading()
	currentHeading := g.nmeaData.CompassHeading

	if !math.IsNaN(lastHeading) && math.IsNaN(currentHeading) {
		return lastHeading, nil
	}

	if !math.IsNaN(currentHeading) && currentHeading != lastHeading {
		g.lastCompassHeading.SetLastCompassHeading(currentHeading)
	}

	return currentHeading, nil
}

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *CachedData) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nmeaData.FixQuality, nil
}

// ReadSatsInView returns the number of satellites in view.
func (g *CachedData) ReadSatsInView(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nmeaData.SatsInView, nil
}

// Properties returns what movement sensor capabilities we have.
func (g *CachedData) Properties(
	ctx context.Context, extra map[string]interface{},
) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
		CompassHeadingSupported: true,
	}, nil
}

// findDistance calculates the distance between two points on Earth.
// lat1, lon1: Latitude and Longitude of point 1 (in decimal degrees)
// lat2, lon2: Latitude and Longitude of point 2 (in decimal degrees)
// returns the distance in meters
func findDistance(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad, lon1Rad := utils.DegToRad(lat1), utils.DegToRad(lon1)
	lat2Rad, lon2Rad := utils.DegToRad(lat2), utils.DegToRad(lon2)

	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distanceKm := earthRadiusKm * c
	return distanceKm * 1000 // convert km to meters
}

// calculateCompassDegreeError calculates the compass degree error
// of two geo points.
func (g *CachedData) calculateCompassDegreeError() float64 {
	firstPos := g.lastPosition.GetLastPosition()
	secondPos := g.nmeaData.Location
	adjacent := findDistance(firstPos.Lat(), firstPos.Lng(), secondPos.Lat(), secondPos.Lng())
	radius := 5.0
	if g.nmeaData.FixQuality >= 4 {
		radius = 0.1
	}
	// math.Atan2 returns the angle in radians, so we convert it to degrees.
	thetaRadians := math.Atan2(radius, adjacent)
	thetaDegrees := utils.RadToDeg(thetaRadians)
	return thetaDegrees
}
